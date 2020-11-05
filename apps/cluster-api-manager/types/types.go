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
	Spec     interface{}          `json:"spec,omitempty"`
	List     interface{}          `json:"list,omitempty"`
	Status   interface{}          `json:"status,omitempty"`
}

type Endpoints []struct {
	EndpointPath string
	HandlerFunc  http.HandlerFunc
	HttpMethods  []string
}

type SetupSpec struct {
	User          string   `json:"user"`
	UserLowercase string   `json:"-"`
	Guests        []string `json:"guests"`
	Repos         []string `json:"repos"`
	Timezone      string   `json:"timezone"`
	Fullname      string   `json:"fullname"`
	Email         string   `json:"email"`
	HumacsVersion string   `json:"-"`
}
