package main

import (
	"context"
	"fmt"
	"log"
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
	AppBuildVersion = "0.0.0"
	AppBuildHash    = "???"
	AppBuildDate    = "???"
	AppBuildMode    = "development"
)

type reconciler struct {
	clientset        *kubernetes.Clientset
	dynamicClientset dynamic.Interface
	restConfig       *rest.Config
	targetNamespace  string
}

func (r *reconciler) init() {
	err, clientset := camk8s.Client()
	if err != nil {
		log.Panicln(err)
		return
	}
	if clientset == nil {
		log.Panicln("clientset is nil")
		return
	}
	r.clientset = clientset

	err, dynamicClientset := camk8s.DynamicClient()
	if err != nil {
		log.Panicln(err)
		return
	}
	r.dynamicClientset = dynamicClientset

	err, restConfig := camk8s.RestClient()
	if err != nil {
		log.Panicln(err)
		return
	}
	r.restConfig = restConfig

	r.targetNamespace = common.GetTargetNamespace()
}

func (r *reconciler) getClustersList() (err error, clusters []clusterAPIv1alpha3.Cluster) {
	groupVersion := clusterAPIv1alpha3.GroupVersion
	groupVersionResource := schema.GroupVersionResource{Version: groupVersion.Version, Group: "cluster.x-k8s.io", Resource: "clusters"}
	items, err := r.dynamicClientset.Resource(groupVersionResource).Namespace(r.targetNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to list Cluster, %#v", err), clusters
	}
	for _, item := range items.Items {
		var c clusterAPIv1alpha3.Cluster
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, c)
		if err != nil {
			return fmt.Errorf("Failed to restructure %T", c), []clusterAPIv1alpha3.Cluster{}
		}
		if c.ObjectMeta.Labels["io.sharing.pair"] != "instance" {
			log.Printf("Not using object %s/%T/%s - not an instance managed by sharingio/pair\n", r.targetNamespace, c, c.ObjectMeta.Name)
			continue
		}
		clusters = append(clusters, c)
	}
	return err, clusters
}

func main() {
	log.Printf("launching reconciler (%v, %v, %v, %v)\n", AppBuildVersion, AppBuildHash, AppBuildDate, AppBuildMode)
	envFile := common.GetAppEnvFile()
	_ = godotenv.Load(envFile)

	var r *reconciler
	r.init()

	for {
		err, clusters := r.getClustersList()
		if err != nil {
			panic(err)
		}
		for _, c := range clusters {
			fmt.Println(c.ObjectMeta.Name)
		}

		time.Sleep(60 * time.Second)
	}

}
