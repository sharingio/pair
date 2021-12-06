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
func Client() (clientset *kubernetes.Clientset, err error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return clientset, err
	}
	config.UserAgent = defaultKubeClientUserAgent
	config.QPS = 500
	config.Burst = 1000
	clientset, err = kubernetes.NewForConfig(config)
	return clientset, err
}

// DynamicClient ...
// return a dynamic client
func DynamicClient() (clientset dynamic.Interface, err error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return clientset, err
	}
	config.UserAgent = defaultKubeClientUserAgent
	clientset, err = dynamic.NewForConfig(config)
	return clientset, err
}

// RestClient ...
// return a rest client
func RestClient() (config *rest.Config, err error) {
	config, err = rest.InClusterConfig()
	if err != nil {
		return &rest.Config{}, err
	}
	config.UserAgent = defaultKubeClientUserAgent
	return config, nil
}
