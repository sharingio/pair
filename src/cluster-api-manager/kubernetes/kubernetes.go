package kubernetes

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func Client() (err error, clientset *kubernetes.Clientset) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return err, clientset
	}
	clientset, err = kubernetes.NewForConfig(config)
	return err, clientset
}
