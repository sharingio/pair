/*
	route related
*/

package routes

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/sharingio/pair/src/cluster-api-manager/common"
	"github.com/sharingio/pair/src/cluster-api-manager/instances"
	"github.com/sharingio/pair/src/cluster-api-manager/types"
)

func PostInstance(w http.ResponseWriter, r *http.Request) {
	response := "Failed to create instance"
	responseCode := http.StatusInternalServerError

	var instance instances.InstanceSpec
	body, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(body, &instance)

	err, instanceCreated := instances.Create(instance)
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
