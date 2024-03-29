/*
	route related
*/

package routes

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	// networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/sharingio/pair/apps/cluster-api-manager/common"
	"github.com/sharingio/pair/apps/cluster-api-manager/instances"
	"github.com/sharingio/pair/apps/cluster-api-manager/types"
)

// GetInstanceKubernetes ...
// handler for getting a kubernetes instance type
func GetInstanceKubernetes(dynamicClient dynamic.Interface, clientset *kubernetes.Clientset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responseCode := http.StatusInternalServerError

		vars := mux.Vars(r)
		name := vars["name"]

		instance, err := instances.KubernetesGet(name, dynamicClient, clientset)
		if instance.Spec.Name == "" && err == nil {
			responseCode = http.StatusNotFound
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: "Resource not found",
				},
				Spec:   instances.InstanceSpec{},
				Status: instances.InstanceStatus{},
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}
		if err != nil {
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: err.Error(),
				},
				Spec:   instances.InstanceSpec{},
				Status: instances.InstanceStatus{},
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}
		responseCode = http.StatusOK
		JSONresp := types.JSONMessageResponse{
			Metadata: types.JSONResponseMetadata{
				Response: "Fetched Kubernetes instance",
			},
			Spec:   instance.Spec,
			Status: instance.Status,
		}
		common.JSONResponse(r, w, responseCode, JSONresp)
	}
}

// ListInstances ...
// handler for all instances
func ListInstances(dynamicClient dynamic.Interface, clientset *kubernetes.Clientset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := "Listing all instances"
		responseCode := http.StatusInternalServerError

		instanceFilterUsername := r.FormValue("username")
		instanceFilterType := r.FormValue("type")
		options := instances.InstanceListOptions{
			Filter: instances.InstanceFilter{
				Username: instanceFilterUsername,
				Type:     instances.InstanceType(instanceFilterType),
			},
		}

		availableInstances, err := instances.List(dynamicClient, clientset, options)
		if err != nil {
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: err.Error(),
				},
				List: []instances.Instance{},
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}
		if len(availableInstances) == 0 {
			response = "No instances found"
		}
		responseCode = http.StatusOK
		JSONresp := types.JSONMessageResponse{
			Metadata: types.JSONResponseMetadata{
				Response: response,
			},
			List: availableInstances,
		}
		common.JSONResponse(r, w, responseCode, JSONresp)
	}
}

// ListInstancesKubernetes ...
// handler for listing Kubernetes instances
func ListInstancesKubernetes(dynamicClient dynamic.Interface, clientset *kubernetes.Clientset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := "Listing all Kubernetes instances"
		responseCode := http.StatusInternalServerError

		instanceFilterUsername := r.FormValue("username")
		options := instances.InstanceListOptions{
			Filter: instances.InstanceFilter{
				Username: instanceFilterUsername,
			},
		}

		availableInstances, err := instances.KubernetesList(dynamicClient, clientset, options)
		if err != nil {
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: err.Error(),
				},
				List: []instances.Instance{},
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}
		if len(availableInstances) == 0 {
			response = "No Kubernetes instances found"
		}
		responseCode = http.StatusOK
		JSONresp := types.JSONMessageResponse{
			Metadata: types.JSONResponseMetadata{
				Response: response,
			},
			List: availableInstances,
		}
		common.JSONResponse(r, w, responseCode, JSONresp)
	}
}

// PostInstance ...
// handler for creating an instance
func PostInstance(dynamicClient dynamic.Interface, clientset *kubernetes.Clientset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responseCode := http.StatusInternalServerError

		var instance instances.InstanceSpec
		body, _ := ioutil.ReadAll(r.Body)
		json.Unmarshal(body, &instance)

		dryRunFormValue := r.FormValue("dryRun")
		options := instances.InstanceCreateOptions{
			DryRun: dryRunFormValue == "true",
		}

		instanceCreated, err := instances.Create(instance, dynamicClient, clientset, options)
		if err != nil {
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: err.Error(),
				},
				Spec:   instances.InstanceSpec{},
				Status: instances.InstanceStatus{},
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}
		responseCode = http.StatusCreated
		JSONresp := types.JSONMessageResponse{
			Metadata: types.JSONResponseMetadata{
				Response: "Creating instance",
			},
			Spec: instanceCreated,
			Status: instances.InstanceStatus{
				Phase: instances.InstanceStatusPhasePending,
			},
		}
		common.JSONResponse(r, w, responseCode, JSONresp)
	}
}

// DeleteInstanceKubernetes ...
// handler for deleting a Kubernetes instance type
func DeleteInstanceKubernetes(dynamicClient dynamic.Interface, clientset *kubernetes.Clientset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responseCode := http.StatusInternalServerError

		vars := mux.Vars(r)
		name := vars["name"]

		instance, err := instances.KubernetesGet(name, dynamicClient, clientset)
		if err != nil {
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: err.Error(),
				},
				Spec:   instances.InstanceSpec{},
				Status: instances.InstanceStatus{},
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}
		if instance.Spec.Name == "" {
			responseCode = http.StatusNotFound
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: "Resource not found",
				},
				Spec:   instances.InstanceSpec{},
				Status: instances.InstanceStatus{},
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}

		err = instances.KubernetesDelete(name, dynamicClient)
		if err != nil {
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: err.Error(),
				},
				Spec:   instances.InstanceSpec{},
				Status: instances.InstanceStatus{},
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}
		responseCode = http.StatusOK
		JSONresp := types.JSONMessageResponse{
			Metadata: types.JSONResponseMetadata{
				Response: "Deleting instance",
			},
			Status: instances.InstanceStatus{
				Phase: instances.InstanceStatusPhaseDeleting,
			},
		}
		common.JSONResponse(r, w, responseCode, JSONresp)
	}
}

// DeleteInstance ...
// handler for deleting an instance
func DeleteInstance(dynamicClient dynamic.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responseCode := http.StatusInternalServerError

		var instance instances.InstanceSpec
		body, _ := ioutil.ReadAll(r.Body)
		json.Unmarshal(body, &instance)

		err := instances.Delete(instance, dynamicClient)
		if err != nil {
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: err.Error(),
				},
				Spec:   instances.InstanceSpec{},
				Status: instances.InstanceStatus{},
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}
		responseCode = http.StatusOK
		JSONresp := types.JSONMessageResponse{
			Metadata: types.JSONResponseMetadata{
				Response: "Deleting instance",
			},
			Status: instances.InstanceStatus{
				Phase: instances.InstanceStatusPhaseDeleting,
			},
		}
		common.JSONResponse(r, w, responseCode, JSONresp)
	}
}

// GetKubernetesKubeconfig ...
// handler for getting an instance's KubeConfig as YAML
func GetKubernetesKubeconfig(kubernetesClientset *kubernetes.Clientset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := "Fetched Kubeconfig for instance"
		responseCode := http.StatusInternalServerError

		vars := mux.Vars(r)
		name := vars["name"]

		kubeconfig, err := instances.KubernetesGetKubeconfigYAML(name, kubernetesClientset)
		if kubeconfig == "" && err == nil {
			responseCode = http.StatusNotFound
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: "Resource not found",
				},
				Spec: clientcmdapi.Config{},
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}
		if err != nil {
			log.Println(err)
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: err.Error(),
				},
				Spec: kubeconfig,
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}
		responseCode = http.StatusOK
		JSONresp := types.JSONMessageResponse{
			Metadata: types.JSONResponseMetadata{
				Response: response,
			},
			Spec: kubeconfig,
		}
		common.JSONResponse(r, w, responseCode, JSONresp)
	}
}

// GetKubernetesTmateSSHSession ...
// handler for getting an instance's tmate SSH session
func GetKubernetesTmateSSHSession(clientset *kubernetes.Clientset, restConfig *rest.Config, dynamicClientSet dynamic.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := "Fetched Tmate session for instance"
		responseCode := http.StatusInternalServerError

		vars := mux.Vars(r)
		name := vars["name"]

		instance, err := instances.KubernetesGet(name, dynamicClientSet, clientset)
		if instance.Spec.Name == "" && err == nil {
			responseCode = http.StatusNotFound
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: "Resource not found",
				},
				Spec:   instances.InstanceSpec{},
				Status: instances.InstanceStatus{},
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}

		instance.Spec.Setup.UserLowercase = strings.ToLower(instance.Spec.Setup.User)
		session, err := instances.KubernetesGetTmateSSHSession(clientset, name, instance.Spec.Setup.UserLowercase)
		notFound := err != nil && (strings.Contains(err.Error(), "Failed to get Kubernetes cluster Kubeconfig") ||
			strings.Contains(err.Error(), "not found"))
		if firstSnippit := strings.Split(session, " "); firstSnippit[0] != "ssh" && err == nil || notFound {
			responseCode = http.StatusNotFound
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: "Resource not found",
				},
				Spec: "",
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}
		if err != nil {
			log.Println(err)
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: err.Error(),
				},
				Spec: "",
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}
		responseCode = http.StatusOK
		JSONresp := types.JSONMessageResponse{
			Metadata: types.JSONResponseMetadata{
				Response: response,
			},
			Spec: session,
		}
		common.JSONResponse(r, w, responseCode, JSONresp)
	}
}

// GetKubernetesTmateWebSession ...
// handler for getting an instance's tmate web session
func GetKubernetesTmateWebSession(clientset *kubernetes.Clientset, restConfig *rest.Config, dynamicClientSet dynamic.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := "Fetched Tmate session for instance"
		responseCode := http.StatusInternalServerError

		vars := mux.Vars(r)
		name := vars["name"]

		instance, err := instances.KubernetesGet(name, dynamicClientSet, clientset)
		if instance.Spec.Name == "" && err == nil {
			responseCode = http.StatusNotFound
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: "Resource not found",
				},
				Spec:   instances.InstanceSpec{},
				Status: instances.InstanceStatus{},
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}

		instance.Spec.Setup.UserLowercase = strings.ToLower(instance.Spec.Setup.User)
		session, err := instances.KubernetesGetTmateWebSession(clientset, name, instance.Spec.Setup.UserLowercase)
		notFound := err != nil && (strings.Contains(err.Error(), "Failed to get Kubernetes cluster Kubeconfig") ||
			strings.Contains(err.Error(), "not found"))
		if firstSnippit := strings.Split(session, ":"); firstSnippit[0] != "https" && err == nil || notFound {
			responseCode = http.StatusNotFound
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: "Resource not found",
				},
				Spec: "",
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}
		if err != nil {
			log.Println(err)
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: err.Error(),
				},
				Spec: "",
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}
		responseCode = http.StatusOK
		JSONresp := types.JSONMessageResponse{
			Metadata: types.JSONResponseMetadata{
				Response: response,
			},
			Spec: session,
		}
		common.JSONResponse(r, w, responseCode, JSONresp)
	}
}

// GetKubernetesIngresses ...
// handler for getting an instance's ingresse mappings
func GetKubernetesIngresses(kubernetesClientset *kubernetes.Clientset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := "Fetched Kubeconfig for instance"
		responseCode := http.StatusInternalServerError

		vars := mux.Vars(r)
		name := vars["name"]

		ingresses, err := instances.KubernetesGetInstanceIngresses(kubernetesClientset, name)
		if len(ingresses) == 0 && err == nil {
			responseCode = http.StatusNotFound
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: "Resource not found",
				},
				Spec: []instances.Ingress{},
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}
		if err != nil {
			log.Println(err)
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: err.Error(),
				},
				Spec: []instances.Ingress{},
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}
		responseCode = http.StatusOK
		JSONresp := types.JSONMessageResponse{
			Metadata: types.JSONResponseMetadata{
				Response: response,
			},
			List: ingresses,
		}
		common.JSONResponse(r, w, responseCode, JSONresp)
	}
}

// PostKubernetesDNSManage ...
// handler for initiating DNS management for an instance
func PostKubernetesDNSManage(dynamicClient dynamic.Interface, clientset *kubernetes.Clientset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := "Failed to initiate DNS management"
		responseCode := http.StatusInternalServerError

		vars := mux.Vars(r)
		name := vars["name"]

		instance, err := instances.KubernetesGet(name, dynamicClient, clientset)
		if instance.Spec.Name == "" && err == nil {
			responseCode = http.StatusNotFound
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: "Resource not found",
				},
				Spec:   instances.InstanceSpec{},
				Status: instances.InstanceStatus{},
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}
		if err != nil {
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: err.Error(),
				},
				Spec:   instances.InstanceSpec{},
				Status: instances.InstanceStatus{},
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}

		instance.Spec.Setup.UserLowercase = strings.ToLower(instance.Spec.Setup.User)

		err = instances.KubernetesAddMachineIPToDNS(dynamicClient, name, name)
		if err != nil {
			response = fmt.Sprintf("%v: %v", response, err.Error())
		} else {
			response = "DNS records synced"
			responseCode = http.StatusOK
		}
		JSONresp := types.JSONMessageResponse{
			Metadata: types.JSONResponseMetadata{
				Response: response,
			},
		}
		common.JSONResponse(r, w, responseCode, JSONresp)
	}
}

// PostKubernetesCertManage ...
// handler for initiating certificate management for an instance
func PostKubernetesCertManage(clientset *kubernetes.Clientset, dynamicClient dynamic.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := "Failed to initiate cert management"
		responseCode := http.StatusInternalServerError

		vars := mux.Vars(r)
		name := vars["name"]

		instance, err := instances.KubernetesGet(name, dynamicClient, clientset)
		if instance.Spec.Name == "" && err == nil {
			responseCode = http.StatusNotFound
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: "Resource not found",
				},
				Spec:   instances.InstanceSpec{},
				Status: instances.InstanceStatus{},
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}
		if err != nil {
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: err.Error(),
				},
				Spec:   instances.InstanceSpec{},
				Status: instances.InstanceStatus{},
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}

		instance.Spec.Setup.UserLowercase = strings.ToLower(instance.Spec.Setup.User)

		err = instances.KubernetesAddCertToMachine(clientset, dynamicClient, instance.Spec)
		if err != nil {
			response = fmt.Sprintf("%v: %v", response, err.Error())
		} else {
			response = "Certificate synced"
			responseCode = http.StatusOK
		}
		JSONresp := types.JSONMessageResponse{
			Metadata: types.JSONResponseMetadata{
				Response: response,
			},
		}
		common.JSONResponse(r, w, responseCode, JSONresp)
	}
}

// PostKubernetesUpdateInstanceNodeProviderID handler for updateing Kubernetes Instance Node Provider ID
func PostKubernetesUpdateInstanceNodeProviderID(clientset *kubernetes.Clientset, dynamicClient dynamic.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := "Failed to update Node ProviderID"
		responseCode := http.StatusInternalServerError

		vars := mux.Vars(r)
		name := vars["name"]

		instance, err := instances.KubernetesGet(name, dynamicClient, clientset)
		if instance.Spec.Name == "" && err == nil {
			responseCode = http.StatusNotFound
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: "Resource not found",
				},
				Spec:   instances.InstanceSpec{},
				Status: instances.InstanceStatus{},
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}
		if err != nil {
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: err.Error(),
				},
				Spec:   instances.InstanceSpec{},
				Status: instances.InstanceStatus{},
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}

		err = instances.UpdateInstanceNodeWithProviderID(clientset, dynamicClient, instance.Spec.Name)
		if err != nil {
			response = fmt.Sprintf("%v: %v", response, err.Error())
		} else {
			response = "Updated Node ProviderID"
			responseCode = http.StatusOK
		}
		JSONresp := types.JSONMessageResponse{
			Metadata: types.JSONResponseMetadata{
				Response: response,
			},
		}
		common.JSONResponse(r, w, responseCode, JSONresp)
	}
}

// GetRoot ...
// get root of API
func GetRoot(w http.ResponseWriter, r *http.Request) {
	JSONresp := types.JSONMessageResponse{
		Metadata: types.JSONResponseMetadata{
			Response: "Hit root of webserver",
		},
	}
	common.JSONResponse(r, w, http.StatusOK, JSONresp)
}

// GetAPIHello ...
// example request
func GetAPIHello(w http.ResponseWriter, r *http.Request) {
	JSONresp := types.JSONMessageResponse{
		Metadata: types.JSONResponseMetadata{
			Response: "hello",
		},
	}
	common.JSONResponse(r, w, http.StatusOK, JSONresp)
}

// GetTeapot ...
// who's a little teapot?
func GetTeapot(w http.ResponseWriter, r *http.Request) {
	JSONresp := types.JSONMessageResponse{
		Metadata: types.JSONResponseMetadata{
			Response: "I'm a little teapot",
		},
	}
	common.JSONResponse(r, w, http.StatusTeapot, JSONresp)
}

// APIUnknownEndpoint ...
// generic unknown endpoint response
func APIUnknownEndpoint(w http.ResponseWriter, r *http.Request) {
	common.JSONResponse(r, w, http.StatusNotFound, types.JSONMessageResponse{
		Metadata: types.JSONResponseMetadata{
			Response: "This endpoint doesn't seem to exist.",
		},
	})
}
