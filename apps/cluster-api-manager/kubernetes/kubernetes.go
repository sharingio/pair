package kubernetes

import (
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// user agent for Kubernetes APIServer
var defaultKubeClientUserAgent = "sharingio/pair/cluster-api-manager"

// Client ...
// return a clientset
func Client() (err error, clientset *kubernetes.Clientset) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return err, clientset
	}
	config.UserAgent = defaultKubeClientUserAgent
	config.QPS = 500
	config.Burst = 1000
	clientset, err = kubernetes.NewForConfig(config)
	return err, clientset
}

// DynamicClient ...
// return a dynamic client
func DynamicClient() (err error, clientset dynamic.Interface) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return err, clientset
	}
	config.UserAgent = defaultKubeClientUserAgent
	clientset, err = dynamic.NewForConfig(config)
	return err, clientset
}

// RestClient ...
// return a rest client
func RestClient() (err error, config *rest.Config) {
	config, err = rest.InClusterConfig()
	config.UserAgent = defaultKubeClientUserAgent
	return err, config
}
