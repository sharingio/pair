package routes

import (
	"net/http"

	"github.com/sharingio/pair/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// GetEndpoints ...
// returns endpoints to register
func GetEndpoints(endpointPrefix string, clientset *kubernetes.Clientset, dynamicClient dynamic.Interface, restConfig *rest.Config) types.Endpoints {
	return types.Endpoints{

		// swagger:route GET /hello hello getHello
		//
		// Say hello
		//
		//     Consumes:
		//     - application/json
		//
		//     Produces:
		//     - application/json
		//
		//     Schemes: http
		//
		//     Responses:
		//       200: metaResponse
		{
			EndpointPath: endpointPrefix + "/hello",
			HandlerFunc:  GetAPIHello,
			HttpMethods:  []string{http.MethodGet},
		},

		// swagger:route GET /teapot teapot getTeapot
		//
		// I'm a little teapot
		//
		//     Consumes:
		//     - application/json
		//
		//     Produces:
		//     - application/json
		//
		//     Schemes: http
		//
		//     Responses:
		//       418: metaResponse
		{
			EndpointPath: endpointPrefix + "/teapot",
			HandlerFunc:  GetTeapot,
			HttpMethods:  []string{http.MethodGet},
		},

		// swagger:route GET /instance instance listInstances
		//
		// List all instances
		//
		//     Consumes:
		//     - application/json
		//
		//     Produces:
		//     - application/json
		//
		//     Schemes: http
		//
		//     Responses:
		//       200: instanceList
		//       500: failure
		{
			EndpointPath: endpointPrefix + "/instance",
			HandlerFunc:  ListInstances(dynamicClient),
			HttpMethods:  []string{http.MethodGet},
		},

		// swagger:route GET /instance/kubernetes instance listInstancesKubernetes
		//
		// List all Kubernetes instances
		//
		//     Consumes:
		//     - application/json
		//
		//     Produces:
		//     - application/json
		//
		//     Schemes: http
		//
		//     Responses:
		//       200: instanceList
		//       500: failure
		{
			EndpointPath: endpointPrefix + "/instance/kubernetes",
			HandlerFunc:  ListInstancesKubernetes(dynamicClient),
			HttpMethods:  []string{http.MethodGet},
		},

		// swagger:route GET /instance/kubernetes/{name} instance getInstanceKubernetes
		//
		// get a Kubernetes instance
		//
		//     Consumes:
		//     - application/json
		//
		//     Produces:
		//     - application/json
		//
		//     Schemes: http
		//
		//     Responses:
		//       200: instance
		//       500: failure
		{
			EndpointPath: endpointPrefix + "/instance/kubernetes/{name}",
			HandlerFunc:  GetInstanceKubernetes(dynamicClient),
			HttpMethods:  []string{http.MethodGet},
		},

		// swagger:route GET /instance/kubernetes/{name}/kubeconfig instance getInstanceKubernetesKubeconfig
		//
		// get a kubeconfig for a Kubernetes instance
		//
		//     Consumes:
		//     - application/json
		//
		//     Produces:
		//     - application/json
		//
		//     Schemes: http
		//
		//     Responses:
		//       200: instanceData
		//       500: failure
		{
			EndpointPath: endpointPrefix + "/instance/kubernetes/{name}/kubeconfig",
			HandlerFunc:  GetKubernetesKubeconfig(clientset),
			HttpMethods:  []string{http.MethodGet},
		},

		// swagger:route GET /instance/kubernetes/{name}/ingresses instance getInstanceKubernetesIngresses
		//
		// get available ingresses for a Kubernetes instance
		//
		//     Consumes:
		//     - application/json
		//
		//     Produces:
		//     - application/json
		//
		//     Schemes: http
		//
		//     Responses:
		//       200: instanceIngresses
		//       500: failure
		{
			EndpointPath: endpointPrefix + "/instance/kubernetes/{name}/ingresses",
			HandlerFunc:  GetKubernetesIngresses(clientset),
			HttpMethods:  []string{http.MethodGet},
		},

		// swagger:route POST /instance/kubernetes/{name}/certmanage instance getInstanceKubernetesCertmanage
		//
		// initiate certificate management for an instance
		//
		//     Consumes:
		//     - application/json
		//
		//     Produces:
		//     - application/json
		//
		//     Schemes: http
		//
		//     Responses:
		//       200: metaResponse
		//       500: failure
		{
			EndpointPath: endpointPrefix + "/instance/kubernetes/{name}/certmanage",
			HandlerFunc:  PostKubernetesCertManage(clientset, dynamicClient),
			HttpMethods:  []string{http.MethodPost},
		},

		// swagger:route POST /instance/kubernetes/{name}/dnsmanage instance getInstanceKubernetesDNSmanage
		//
		// initiate DNS management for an instance
		//
		//     Consumes:
		//     - application/json
		//
		//     Produces:
		//     - application/json
		//
		//     Schemes: http
		//
		//     Responses:
		//       200: metaResponse
		//       500: failure
		{
			EndpointPath: endpointPrefix + "/instance/kubernetes/{name}/dnsmanage",
			HandlerFunc:  PostKubernetesDNSManage(dynamicClient),
			HttpMethods:  []string{http.MethodPost},
		},

		// swagger:route GET /instance/kubernetes/{name}/tmate instance getInstanceKubernetesTmate
		//
		// get a tmate SSH sesion for an instance
		//
		//     Consumes:
		//     - application/json
		//
		//     Produces:
		//     - application/json
		//
		//     Schemes: http
		//
		//     Responses:
		//       200: instanceData
		//       500: failure
		{
			EndpointPath: endpointPrefix + "/instance/kubernetes/{name}/tmate",
			HandlerFunc:  GetKubernetesTmateSSHSession(clientset, restConfig, dynamicClient),
			HttpMethods:  []string{http.MethodGet},
		},

		// swagger:route GET /instance/kubernetes/{name}/tmate/ssh instance getInstanceKubernetesTmateSSH
		//
		// get a tmate SSH sesion for an instance
		//
		//     Consumes:
		//     - application/json
		//
		//     Produces:
		//     - application/json
		//
		//     Schemes: http
		//
		//     Responses:
		//       200: instanceData
		//       500: failure
		{
			EndpointPath: endpointPrefix + "/instance/kubernetes/{name}/tmate/ssh",
			HandlerFunc:  GetKubernetesTmateSSHSession(clientset, restConfig, dynamicClient),
			HttpMethods:  []string{http.MethodGet},
		},

		// swagger:route GET /instance/kubernetes/{name}/tmate/web instance getInstanceKubernetesTmateWeb
		//
		// get a tmate web sesion for an instance
		//
		//     Consumes:
		//     - application/json
		//
		//     Produces:
		//     - application/json
		//
		//     Schemes: http
		//
		//     Responses:
		//       200: instanceData
		//       500: failure
		{
			EndpointPath: endpointPrefix + "/instance/kubernetes/{name}/tmate/web",
			HandlerFunc:  GetKubernetesTmateWebSession(clientset, restConfig, dynamicClient),
			HttpMethods:  []string{http.MethodGet},
		},

		// swagger:route POST /instance instance postInstance
		//
		// creates an instance
		//
		//     Consumes:
		//     - application/json
		//
		//     Produces:
		//     - application/json
		//
		//     Schemes: http
		//
		//     Responses:
		//       200: instance
		//       500: failure
		{
			EndpointPath: endpointPrefix + "/instance",
			HandlerFunc:  PostInstance(dynamicClient, clientset),
			HttpMethods:  []string{http.MethodPost},
		},

		// swagger:route DELETE /instance/kubernetes/{name} instance deleteInstanceKubernetes
		//
		// delete a Kubernetes instance
		//
		//     Consumes:
		//     - application/json
		//
		//     Produces:
		//     - application/json
		//
		//     Schemes: http
		//
		//     Responses:
		//       200: instance
		//       500: failure
		{
			EndpointPath: endpointPrefix + "/instance/kubernetes/{name}",
			HandlerFunc:  DeleteInstanceKubernetes(dynamicClient),
			HttpMethods:  []string{http.MethodDelete},
		},

		// swagger:route DELETE /instance instance deleteInstance
		//
		// delete an instance
		//
		//     Consumes:
		//     - application/json
		//
		//     Produces:
		//     - application/json
		//
		//     Schemes: http
		//
		//     Responses:
		//       200: instance
		//       500: failure
		{
			EndpointPath: endpointPrefix + "/instance",
			HandlerFunc:  DeleteInstance(dynamicClient),
			HttpMethods:  []string{http.MethodDelete},
		},
	}
}
