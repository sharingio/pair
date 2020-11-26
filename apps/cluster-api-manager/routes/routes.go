/*
	route related
*/

package routes

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/sharingio/pair/common"
	"github.com/sharingio/pair/instances"
	"github.com/sharingio/pair/types"
)

func GetInstanceKubernetes(dynamicClient dynamic.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responseCode := http.StatusInternalServerError

		vars := mux.Vars(r)
		name := vars["name"]

		err, instance := instances.KubernetesGet(name, dynamicClient)
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

func ListInstancesKubernetes(dynamicClient dynamic.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := "Listing all Kubernetes instances"
		responseCode := http.StatusInternalServerError

		instanceFilterUsername := r.FormValue("username")
		options := instances.InstanceListOptions{
			Filter: instances.InstanceFilter{
				Username: instanceFilterUsername,
			},
		}

		err, availableInstances := instances.KubernetesList(dynamicClient, options)
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

		err, instanceCreated := instances.Create(instance, dynamicClient, clientset, options)
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

func DeleteInstanceKubernetes(dynamicClient dynamic.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responseCode := http.StatusInternalServerError

		vars := mux.Vars(r)
		name := vars["name"]

		err, instance := instances.KubernetesGet(name, dynamicClient)
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

func GetKubernetesKubeconfig(kubernetesClientset *kubernetes.Clientset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := "Fetched Kubeconfig for instance"
		responseCode := http.StatusInternalServerError

		vars := mux.Vars(r)
		name := vars["name"]

		err, kubeconfig := instances.KubernetesGetKubeconfigYAML(name, kubernetesClientset)
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

func GetKubernetesTmateSSHSession(clientset *kubernetes.Clientset, restConfig *rest.Config, dynamicClientSet dynamic.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := "Fetched Tmate session for instance"
		responseCode := http.StatusInternalServerError

		vars := mux.Vars(r)
		name := vars["name"]

		err, instance := instances.KubernetesGet(name, dynamicClientSet)
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
		err, session := instances.KubernetesGetTmateSSHSession(clientset, name, instance.Spec.Setup.UserLowercase)
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

func GetKubernetesTmateWebSession(clientset *kubernetes.Clientset, restConfig *rest.Config, dynamicClientSet dynamic.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := "Fetched Tmate session for instance"
		responseCode := http.StatusInternalServerError

		vars := mux.Vars(r)
		name := vars["name"]

		err, instance := instances.KubernetesGet(name, dynamicClientSet)
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
		err, session := instances.KubernetesGetTmateWebSession(clientset, name, instance.Spec.Setup.UserLowercase)
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

func GetKubernetesIngresses(kubernetesClientset *kubernetes.Clientset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := "Fetched Kubeconfig for instance"
		responseCode := http.StatusInternalServerError

		vars := mux.Vars(r)
		name := vars["name"]

		err, ingresses := instances.KubernetesGetInstanceIngresses(kubernetesClientset, name)
		if len(ingresses.Items) == 0 && err == nil {
			responseCode = http.StatusNotFound
			JSONresp := types.JSONMessageResponse{
				Metadata: types.JSONResponseMetadata{
					Response: "Resource not found",
				},
				Spec: networkingv1.IngressList{},
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
				Spec: networkingv1.IngressList{},
			}
			common.JSONResponse(r, w, responseCode, JSONresp)
			return
		}
		responseCode = http.StatusOK
		JSONresp := types.JSONMessageResponse{
			Metadata: types.JSONResponseMetadata{
				Response: response,
			},
			Spec: ingresses,
		}
		common.JSONResponse(r, w, responseCode, JSONresp)
	}
}

func PostKubernetesDNSManage(dynamicClient dynamic.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := "Failed to initiate DNS management"
		responseCode := http.StatusInternalServerError

		vars := mux.Vars(r)
		name := vars["name"]

		err, instance := instances.KubernetesGet(name, dynamicClient)
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

		_ = instances.KubernetesAddMachineIPToDNS(dynamicClient, name, instance.Spec.Setup.UserLowercase)
		response = "Initiated DNS management"
		responseCode = http.StatusOK
		JSONresp := types.JSONMessageResponse{
			Metadata: types.JSONResponseMetadata{
				Response: response,
			},
		}
		common.JSONResponse(r, w, responseCode, JSONresp)
	}
}

func PostKubernetesCertManage(clientset *kubernetes.Clientset, dynamicClient dynamic.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := "Failed to initiate cert management"
		responseCode := http.StatusInternalServerError

		vars := mux.Vars(r)
		name := vars["name"]

		err, instance := instances.KubernetesGet(name, dynamicClient)
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

		_ = instances.KubernetesAddCertToMachine(clientset, dynamicClient, name, instance.Spec.Setup.UserLowercase)
		response = "Initiated cert management"
		responseCode = http.StatusOK
		JSONresp := types.JSONMessageResponse{
			Metadata: types.JSONResponseMetadata{
				Response: response,
			},
		}
		common.JSONResponse(r, w, responseCode, JSONresp)
	}
}

func APIroot(w http.ResponseWriter, r *http.Request) {
	// root of API
	JSONresp := types.JSONMessageResponse{
		Metadata: types.JSONResponseMetadata{
			Response: "Hit root of webserver",
		},
	}
	common.JSONResponse(r, w, http.StatusOK, JSONresp)
}

func GetAPIHello(w http.ResponseWriter, r *http.Request) {
	// root of API
	JSONresp := types.JSONMessageResponse{
		Metadata: types.JSONResponseMetadata{
			Response: "hello",
		},
	}
	common.JSONResponse(r, w, http.StatusOK, JSONresp)
}

func GetTeapot(w http.ResponseWriter, r *http.Request) {
	// root of API
	JSONresp := types.JSONMessageResponse{
		Metadata: types.JSONResponseMetadata{
			Response: "I'm a little teapot",
		},
	}
	common.JSONResponse(r, w, http.StatusTeapot, JSONresp)
}

func APIUnknownEndpoint(w http.ResponseWriter, r *http.Request) {
	common.JSONResponse(r, w, http.StatusNotFound, types.JSONMessageResponse{
		Metadata: types.JSONResponseMetadata{
			Response: "This endpoint doesn't seem to exist.",
		},
	})
}
