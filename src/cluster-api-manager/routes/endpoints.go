package routes

import (
	"net/http"

	"github.com/sharingio/pair/src/cluster-api-manager/types"
)

func GetEndpoints(endpointPrefix string) types.Endpoints {
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
			HandlerFunc:  PostInstance,
			HttpMethods:  []string{http.MethodPost},
		},
	}
}
