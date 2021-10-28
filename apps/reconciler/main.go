package main

import (
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/sharingio/pair/apps/cluster-api-manager/common"
	camk8s "github.com/sharingio/pair/apps/cluster-api-manager/kubernetes"

	"github.com/jetstack/cert-manager/pkg/util/pki"
	corev1 "k8s.io/api/core/v1"
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
	defaultSleepTime = 60
)

type reconciler struct {
	clientset             *kubernetes.Clientset
	dynamicClientset      dynamic.Interface
	restConfig            *rest.Config
	targetNamespace       string
	clusterAPIManagerHost string
	sleepTime             int
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
	sleepTimeString := common.GetEnvOrDefault("APP_SLEEP_TIME", "60")
	sleepTime, _ := strconv.Atoi(sleepTimeString)
	if sleepTime == 0 {
		sleepTime = defaultSleepTime
	}

	return reconciler{
		clientset:             clientset,
		dynamicClientset:      dynamicClientset,
		restConfig:            restConfig,
		targetNamespace:       targetNamespace,
		clusterAPIManagerHost: clusterAPIManagerHost,
		sleepTime:             sleepTime,
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

func (r *reconciler) getCertForInstance(name string) (certificate *x509.Certificate, err error) {
	templatedSecretName := fmt.Sprintf("%v-tls", name)
	secret, err := r.clientset.CoreV1().Secrets(r.targetNamespace).Get(context.TODO(), templatedSecretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("Error: secret not found '%v-tls'", name)
	}
	if err != nil {
		log.Printf("%#v\n", err)
		return nil, fmt.Errorf("Failed to get Secret '%v' in namespace '%v', %#v", templatedSecretName, r.targetNamespace, err)
	}
	return pki.DecodeX509CertificateBytes(secret.Data[corev1.TLSCertKey])
}

// TODO find when cert-manager will normally try to get a new cert before it expire
// currently using 5 days before
func (r *reconciler) isCertExpired(name string) (expired bool, err error) {
	certificate, err := r.getCertForInstance(name)
	if certificate == nil {
		return false, err
	}
	fmt.Println(certificate.NotAfter)
	return certificate.NotAfter.After(time.Now().AddDate(0, 0, -5)), err
}

func (r *reconciler) RemoveExpiredCertificate(name string) (err error) {
	templatedSecretName := fmt.Sprintf("%v-tls", name)
	log.Printf("Checking for expired TLS cert for '%v'\n", name)
	expired, err := r.isCertExpired(name)
	if expired == true {
		log.Printf("TLS cert for '%v' has expired, now deleting '%v-tls'\n", name, name)
		err = r.clientset.CoreV1().Secrets(r.targetNamespace).Delete(context.TODO(), templatedSecretName, metav1.DeleteOptions{})
	}
	log.Printf("TLS cert for '%v' has not expired\n", name)
	return nil
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
				err := r.RemoveExpiredCertificate(c.ObjectMeta.Name)
				if err != nil {
					log.Printf("Error with certificates '%v'\n", err)
				}
			}
		}

		log.Printf("Sleeping for %v seconds", r.sleepTime)
		time.Sleep(time.Duration(r.sleepTime) * time.Second)
	}
}
