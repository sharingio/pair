package routes

import (
	"net/http"

	"github.com/sharingio/pair/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func GetEndpoints(endpointPrefix string, clientset *kubernetes.Clientset, dynamicClient dynamic.Interface, restConfig *rest.Config) types.Endpoints {
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
			HandlerFunc:  ListInstancesKubernetes(dynamicClient),
			HttpMethods:  []string{http.MethodGet},
		},
		{
			EndpointPath: endpointPrefix + "/instance/kubernetes/{name}",
			HandlerFunc:  GetInstanceKubernetes(dynamicClient),
			HttpMethods:  []string{http.MethodGet},
		},
		{
			EndpointPath: endpointPrefix + "/instance/kubernetes/{name}/kubeconfig",
			HandlerFunc:  GetKubernetesKubeconfig(clientset),
			HttpMethods:  []string{http.MethodGet},
		},
		{
			EndpointPath: endpointPrefix + "/instance/kubernetes/{name}/ingresses",
			HandlerFunc:  GetKubernetesIngresses(clientset),
			HttpMethods:  []string{http.MethodGet},
		},
		{
			EndpointPath: endpointPrefix + "/instance/kubernetes/{name}/tmate",
			HandlerFunc:  GetKubernetesTmateSSHSession(clientset, restConfig, dynamicClient),
			HttpMethods:  []string{http.MethodGet},
		},
		{
			EndpointPath: endpointPrefix + "/instance/kubernetes/{name}/tmate/ssh",
			HandlerFunc:  GetKubernetesTmateSSHSession(clientset, restConfig, dynamicClient),
			HttpMethods:  []string{http.MethodGet},
		},
		{
			EndpointPath: endpointPrefix + "/instance/kubernetes/{name}/tmate/web",
			HandlerFunc:  GetKubernetesTmateWebSession(clientset, restConfig, dynamicClient),
			HttpMethods:  []string{http.MethodGet},
		},
		{
			EndpointPath: endpointPrefix + "/instance",
			HandlerFunc:  PostInstance(dynamicClient),
			HttpMethods:  []string{http.MethodPost},
		},
		{
			EndpointPath: endpointPrefix + "/instance/kubernetes/{name}",
			HandlerFunc:  DeleteInstanceKubernetes(dynamicClient),
			HttpMethods:  []string{http.MethodDelete},
		},
		{
			EndpointPath: endpointPrefix + "/instance",
			HandlerFunc:  DeleteInstance(dynamicClient),
			HttpMethods:  []string{http.MethodDelete},
		},
	}
}
