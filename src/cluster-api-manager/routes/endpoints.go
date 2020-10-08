package routes

import (
	"net/http"

	"github.com/sharingio/pair/src/cluster-api-manager/types"
	"k8s.io/client-go/dynamic"
)

func GetEndpoints(endpointPrefix string, kubernetesClientset dynamic.Interface) types.Endpoints {
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
			EndpointPath: endpointPrefix + "/instance/kubernetes",
			HandlerFunc:  ListInstancesKubernetes(kubernetesClientset),
			HttpMethods:  []string{http.MethodGet},
		},
		{
			EndpointPath: endpointPrefix + "/instance/kubernetes/{name}",
			HandlerFunc:  GetInstanceKubernetes(kubernetesClientset),
			HttpMethods:  []string{http.MethodGet},
		},
		{
			EndpointPath: endpointPrefix + "/instance/kubernetes/{name}/kubeconfig",
			HandlerFunc:  GetKubernetesKubeconfig(kubernetesClientset),
			HttpMethods:  []string{http.MethodGet},
		},
		{
			EndpointPath: endpointPrefix + "/instance",
			HandlerFunc:  PostInstance(kubernetesClientset),
			HttpMethods:  []string{http.MethodPost},
		},
		{
			EndpointPath: endpointPrefix + "/instance/kubernetes/{name}",
			HandlerFunc:  DeleteInstanceKubernetes(kubernetesClientset),
			HttpMethods:  []string{http.MethodDelete},
		},
		{
			EndpointPath: endpointPrefix + "/instance",
			HandlerFunc:  DeleteInstance(kubernetesClientset),
			HttpMethods:  []string{http.MethodDelete},
		},
	}
}
