package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/joho/godotenv"
	"github.com/sharingio/pair/apps/cluster-api-manager/common"
	camk8s "github.com/sharingio/pair/apps/cluster-api-manager/kubernetes"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clusterAPIv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

var (
	AppBuildVersion            = "0.0.0"
	AppBuildHash               = "???"
	AppBuildDate               = "???"
	AppBuildMode               = "development"
	endpointsForReconciliation = []string{
		"certmanage",
		"dnsmanage",
		"syncProviderID",
	}
)

type reconciler struct {
	clientset             *kubernetes.Clientset
	dynamicClientset      dynamic.Interface
	restConfig            *rest.Config
	targetNamespace       string
	clusterAPIManagerHost string
}

func NewReconciler() (r reconciler, err error) {
	err, clientset := camk8s.Client()
	if err != nil {
		log.Panicln(err)
		return
	}
	if clientset == nil {
		log.Panicln("clientset is nil")
		return
	}

	err, dynamicClientset := camk8s.DynamicClient()
	if err != nil {
		log.Panicln(err)
		return
	}

	err, restConfig := camk8s.RestClient()
	if err != nil {
		log.Panicln(err)
		return
	}

	targetNamespace := common.GetTargetNamespace()
	clusterAPIManagerHost := common.GetEnvOrDefault("APP_CLUSTER_API_MANAGER_HOST", "http://sharingio-pair-clusterapimanager:8080")

	return reconciler{
		clientset:             clientset,
		dynamicClientset:      dynamicClientset,
		restConfig:            restConfig,
		targetNamespace:       targetNamespace,
		clusterAPIManagerHost: clusterAPIManagerHost,
	}, err
}

func (r *reconciler) getClustersList() (clusters []clusterAPIv1alpha3.Cluster, err error) {
	groupVersion := clusterAPIv1alpha3.GroupVersion
	groupVersionResource := schema.GroupVersionResource{
		Version:  groupVersion.Version,
		Group:    groupVersion.Group,
		Resource: "clusters",
	}
	items, err := r.dynamicClientset.
		Resource(groupVersionResource).
		Namespace(r.targetNamespace).
		List(context.TODO(), metav1.ListOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return clusters, fmt.Errorf("Failed to list Cluster, %#v", err)
	}
	for _, item := range items.Items {
		var c clusterAPIv1alpha3.Cluster
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &c)
		if err != nil {
			return []clusterAPIv1alpha3.Cluster{}, fmt.Errorf("Failed to restructure %T: error: %v", c, err)
		}
		if c.ObjectMeta.Labels["io.sharing.pair"] != "instance" {
			log.Printf("Not using object %s/%T/%s - not an instance managed by sharingio/pair\n", r.targetNamespace, c, c.ObjectMeta.Name)
			continue
		}
		clusters = append(clusters, c)
	}
	return clusters, err
}

func GetClusterAPIManagerEndpoint(url string) (response string, err error) {
	if url[:1] == "/" {
		url = url[1:]
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Accept", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return string(body), err
}

func main() {
	log.Printf("launching reconciler (%v, %v, %v, %v)\n", AppBuildVersion, AppBuildHash, AppBuildDate, AppBuildMode)
	envFile := common.GetAppEnvFile()
	_ = godotenv.Load(envFile)

	r, err := NewReconciler()
	if err != nil {
		panic(err)
	}

list:
	for {
		clusters, err := r.getClustersList()
		if err != nil {
			log.Printf("Failed to list cluster: %s\n", err)
			continue list
		}
		for _, c := range clusters {
			log.Println(c.ObjectMeta.Name)
			for _, endpoint := range endpointsForReconciliation {
				url := fmt.Sprintf("%s/api/instance/kubernetes/%s/%s", r.clusterAPIManagerHost, c.ObjectMeta.Name, endpoint)
				log.Printf("Trying cluster-api-manager endpoint '%s' (%s)\n", endpoint, url)
				go func() {
					resp, err := GetClusterAPIManagerEndpoint(url)
					if err != nil {
						log.Printf("Error from cluster-api-manager endpoint '%s'\n", err)
					}
					log.Printf("Response from cluster-api-manager endpoint '%s'\n", endpoint, resp)
				}()
			}
		}

		time.Sleep(60 * time.Second)
	}
}
