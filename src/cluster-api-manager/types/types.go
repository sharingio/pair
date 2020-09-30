/*
	handle all types used by API
*/

package types

import (
	"net/http"
)

type JSONResponseMetadata struct {
	URL       string `json:"selfLink"`
	Version   string `json:"version"`
	RequestId string `json:"requestId"`
	Timestamp int64  `json:"timestamp"`
	Response  string `json:"response"`
}

type JSONMessageResponse struct {
	Metadata JSONResponseMetadata `json:"metadata"`
	Spec     interface{}          `json:"spec"`
	Status   interface{}          `json:"status"`
}

type Endpoints []struct {
	EndpointPath string
	HandlerFunc  http.HandlerFunc
	HttpMethods   []string
}

type SetupSpec struct {
	User     string   `json:"user"`
	Guests   []string `json:"guests"`
	Repos    []string `json:"json"`
	Timezone string   `json:"timezone"`
}
