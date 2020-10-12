package kubernetes

import (
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var defaultKubeClientUserAgent = "sharingio/pair/cluster-api-manager"

func Client() (err error, clientset *kubernetes.Clientset) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return err, clientset
	}
	config.UserAgent = defaultKubeClientUserAgent
	clientset, err = kubernetes.NewForConfig(config)
	return err, clientset
}

func DynamicClient() (err error, clientset dynamic.Interface) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return err, clientset
	}
	config.UserAgent = defaultKubeClientUserAgent
	clientset, err = dynamic.NewForConfig(config)
	return err, clientset
}

func RestClient() (err error, config *rest.Config) {
	config, err = rest.InClusterConfig()
	return err, config
}
