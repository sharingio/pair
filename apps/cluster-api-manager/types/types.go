/*
	handle all types used by API
*/

package types

import (
	"net/http"
)

// JSONResponseMetadata ...
// metadata fields in responses
type JSONResponseMetadata struct {
	URL       string `json:"selfLink"`
	Version   string `json:"version"`
	RequestId string `json:"requestId"`
	Timestamp int64  `json:"timestamp"`
	Response  string `json:"response"`
}

// JSONMessageResponse ...
// generic JSON response
type JSONMessageResponse struct {
	Metadata JSONResponseMetadata `json:"metadata"`
	Spec     interface{}          `json:"spec,omitempty"`
	List     interface{}          `json:"list,omitempty"`
	Status   interface{}          `json:"status,omitempty"`
}

// JSONFailure ...
// generic JSON for failure
// swagger:response failure
type JSONFailure struct {
	Metadata JSONResponseMetadata `json:"metadata"`
}

// Endpoints ...
// endpoint slices
type Endpoints []struct {
	EndpointPath string
	HandlerFunc  http.HandlerFunc
	HttpMethods  []string
}

type GitHubEmail struct {
	Email    string `json:"email"`
	Verified bool   `json:"verified"`
}

// SetupSpec ...
// fields for provisioning an instance
type SetupSpec struct {
	User              string              `json:"user"`
	Guests            []string            `json:"guests"`
	Repos             []string            `json:"repos"`
	Timezone          string              `json:"timezone"`
	Fullname          string              `json:"fullname"`
	Email             string              `json:"email"`
	ExtraEmails       []GitHubEmail       `json:"extraEmails"`
	GitHubOAuthToken  string              `json:"githubOAuthToken,omitempty"`
	NoGitHubToken     bool                `json:"noGitHubToken,omitempty"`
	Env               []map[string]string `json:"env,omitempty"`
	BaseDNSName       string              `json:"baseDNSName,omitempty"`
	KubernetesVersion string              `json:"kubernetesVersion,omitempty"`
	HumacsRepository  string              `json:"humacsRepository"`
	HumacsVersion     string              `json:"humacsVersion"`

	GuestsNamesFlat string `json:"-"`
	UserLowercase   string `json:"-"`
}

// MetaResponse ...
// response for task initated
// swagger:response metaResponse
type MetaResponse struct {
	Metadata JSONResponseMetadata `json:"metadata"`
}
