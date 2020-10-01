package routes

import (
	"net/http"

	"github.com/sharingio/pair/src/cluster-api-manager/types"
	"k8s.io/client-go/kubernetes"
)

func GetEndpoints(endpointPrefix string, kubernetesClientset *kubernetes.Clientset) types.Endpoints {
	return types.Endpoints{
		{
			EndpointPath: endpointPrefix + "/hello",
			HandlerFunc:  GetAPIHello,
			HttpMethods:  []string{http.MethodGet},
		},
		{
			EndpointPath: endpointPrefix + "/teapot",
			HandlerFunc:  GetTeapot,
			HttpMethods:  []string{http.MethodGet},
		},
		{
			EndpointPath: endpointPrefix + "/instance",
			HandlerFunc:  PostInstance(kubernetesClientset),
			HttpMethods:  []string{http.MethodPost},
		},
	}
}
