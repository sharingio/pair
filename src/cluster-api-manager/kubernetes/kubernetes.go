package kubernetes

import (
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

func DynamicClient() (err error, clientset dynamic.Interface) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return err, clientset
	}
	clientset, err = dynamic.NewForConfig(config)
	return err, clientset
}
