/*
	route related
*/

package routes

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/sharingio/pair/src/cluster-api-manager/common"
	"github.com/sharingio/pair/src/cluster-api-manager/instances"
	"github.com/sharingio/pair/src/cluster-api-manager/types"
	"io/ioutil"
	"k8s.io/client-go/dynamic"
	"net/http"
)

func ListInstancesKubernetes(kubernetesClientset dynamic.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := "Listing all Kubernetes instances"
		responseCode := http.StatusInternalServerError

		err, availableInstances := instances.KubernetesList(kubernetesClientset)
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
		JSONresp := types.JSONMessageResponse{
			Metadata: types.JSONResponseMetadata{
				Response: response,
			},
			List: availableInstances,
		}
		common.JSONResponse(r, w, responseCode, JSONresp)
	}
}

func PostInstance(kubernetesClientset dynamic.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responseCode := http.StatusInternalServerError

		var instance instances.InstanceSpec
		body, _ := ioutil.ReadAll(r.Body)
		json.Unmarshal(body, &instance)

		err, instanceCreated := instances.Create(instance, kubernetesClientset)
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

func DeleteInstanceKubernetes(kubernetesClientset dynamic.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responseCode := http.StatusInternalServerError

		vars := mux.Vars(r)
		name := vars["name"]

		err := instances.KubernetesDelete(name, kubernetesClientset)
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

func DeleteInstance(kubernetesClientset dynamic.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responseCode := http.StatusInternalServerError

		var instance instances.InstanceSpec
		body, _ := ioutil.ReadAll(r.Body)
		json.Unmarshal(body, &instance)

		err := instances.Delete(instance, kubernetesClientset)
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
