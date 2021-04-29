/*
	common function calls
*/

package common

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/asaskevich/govalidator"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/sharingio/pair/types"
)

// misc vars
var (
	AppBuildVersion = "0.0.0"
	AppBuildHash    = "???"
	AppBuildDate    = "???"
	AppBuildMode    = "development"
	letters         = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
)

// GetEnvOrDefault ...
// return env value or default to value
func GetEnvOrDefault(envName string, defaultValue string) (output string) {
	output = os.Getenv(envName)
	if output == "" {
		output = defaultValue
	}
	return output
}

// GetAppEnvFile ...
// location of an env file to load
func GetAppEnvFile() (output string) {
	return GetEnvOrDefault("APP_ENV_FILE", ".env")
}

// GetAppPort ...
// the port to bind to
func GetAppPort() (output string) {
	return GetEnvOrDefault("APP_PORT", ":8080")
}

// GetPacketProjectID ...
// the project ID to create instances in
func GetPacketProjectID() (id string) {
	return GetEnvOrDefault("APP_PACKET_PROJECT_ID", "")
}

// GetTargetNamespace ...
// the namespace to write Kubernetes objects to
func GetTargetNamespace() (namespace string) {
	return GetEnvOrDefault("APP_TARGET_NAMESPACE", "sharingio-pair-instances")
}

// GetBaseHost ...
// the host where the frontend will be served
func GetBaseHost() (host string) {
	return GetEnvOrDefault("APP_BASE_HOST", "")
}

// Logging ...
// basic request logging middleware
func Logging(next http.Handler) http.Handler {
	// log all requests
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%v %v %v %v %v", r.Method, r.URL, r.Proto, r.Response, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

// JSONResponse ....
// generic JSON response handler
func JSONResponse(r *http.Request, w http.ResponseWriter, code int, output types.JSONMessageResponse) {
	// simpilify sending a JSON response
	output.Metadata.URL = r.RequestURI
	output.Metadata.Timestamp = time.Now().Unix()
	output.Metadata.Version = AppBuildVersion
	response, _ := json.Marshal(output)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

// EncodeObject ...
// encode any object as JSON, returning as bytes
func EncodeObject(obj interface{}) (err error, data []byte) {
	data, err = json.Marshal(obj)
	return err, data
}

// ObjectToUnstructured ...
// convert an object into an unstructured Kubernetes resource
func ObjectToUnstructured(obj interface{}) (err error, unstr *unstructured.Unstructured) {
	err, data := EncodeObject(obj)
	if err != nil {
		return err, unstr
	}
	unstrBody := map[string]interface{}{}
	err = json.Unmarshal(data, &unstrBody)
	return err, &unstructured.Unstructured{Object: unstrBody}
}

// AddRepoGitHubPrefix ...
// add a HTTPS GitHub prefix to a repo link if it's not a valid URL
func AddRepoGitHubPrefix(repos []string) (reposModified []string) {
	for _, repo := range repos {
		if govalidator.IsURL(repo) != true {
			repo = fmt.Sprintf("https://github.com/%s", repo)
		}
		reposModified = append(reposModified, repo)
	}
	return reposModified
}

// ReverseStringArray ...
// reverse the array order
func ReverseStringArray(input []string) []string {
	output := make([]string, 0, len(input))
	for i := len(input) - 1; i >= 0; i-- {
		output = append(output, input[i])
	}
	return output
}

// RandomSequence ...
// generate random string from a set of characters
func RandomSequence(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// ValidateName ...
// validates a name string
func ValidateName(input string) bool {
	re := regexp.MustCompile(`^([a-z0-9]([-a-z0-9]*[a-z0-9])?([a-z0-9]([-a-z0-9]*[a-z0-9])?)*)$`)
	return re.MatchString(input)
}

// ReturnValueOrDefault ...
// returns first string is not empty, otherwise returns second
func ReturnValueOrDefault(first string, second string) string {
	if first != "" {
		return first
	}
	return second
}
