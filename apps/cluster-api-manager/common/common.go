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
	"strconv"
	"strings"
	"time"

	"github.com/asaskevich/govalidator"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/sharingio/pair/apps/cluster-api-manager/types"
)

// misc vars
var (
	AppBuildVersion = "0.0.0"
	AppBuildHash    = "???"
	AppBuildDate    = "???"
	AppBuildMode    = "development"
	// https://github.com/kubernetes/apimachinery/blob/v0.23.3/pkg/util/rand/rand.go#L83
	letters = []rune("bcdfghjklmnpqrstvwxz2456789")
)

// GetEnvOrDefault ...
// returns env value or default to value
func GetEnvOrDefault(envName string, defaultValue string) (output string) {
	output = os.Getenv(envName)
	if output == "" {
		output = defaultValue
	}
	return output
}

// GetAppEnvFile ...
// returns the location of an env file to load
func GetAppEnvFile() (output string) {
	return GetEnvOrDefault("APP_ENV_FILE", ".env")
}

// GetAppPort ...
// returns the port to bind to
func GetAppPort() (output string) {
	return GetEnvOrDefault("APP_PORT", ":8080")
}

// GetPacketProjectID ...
// returns the project ID to create instances in
func GetPacketProjectID() (id string) {
	return GetEnvOrDefault("APP_PACKET_PROJECT_ID", "")
}

// GetTargetNamespace ...
// returns the namespace to write Kubernetes objects to
func GetTargetNamespace() (namespace string) {
	return GetEnvOrDefault("APP_TARGET_NAMESPACE", "sharingio-pair-instances")
}

// GetBaseHost ...
// returns the host where the frontend will be served
func GetBaseHost() (host string) {
	return GetEnvOrDefault("APP_BASE_HOST", "")
}

// GetAdminEmailDomain ...
// returns the admin email domain of accounts
func GetAdminEmailDomain() string {
	return GetEnvOrDefault("APP_ADMIN_EMAIL_DOMAIN", "")
}

// GetGitHubAdminOrgs ...
// returns the GitHub admin orgs
func GetGitHubAdminOrgs() []string {
	return strings.Split(GetEnvOrDefault("APP_GITHUB_ADMIN_ORGS", ""), ",")
}

// GetNonAdminInstanceMaxAmount ...
// returns the max number of instances for non-admins
func GetNonAdminInstanceMaxAmount() int {
	maxString := GetEnvOrDefault("APP_NON_ADMIN_INSTANCE_MAX_AMOUNT", "-1")
	max, _ := strconv.Atoi(maxString)
	return max
}

// GetInstanceContainerRegistryMirrors ...
// returns the url to a mirror container registry
func GetInstanceContainerRegistryMirrors() []string {
	return strings.Split(GetEnvOrDefault("APP_INSTANCE_CONTAINER_REGISTRY_MIRRORS", ""), " ")
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
func EncodeObject(obj interface{}) (data []byte, err error) {
	data, err = json.Marshal(obj)
	return data, err
}

// ObjectToUnstructured ...
// convert an object into an unstructured Kubernetes resource
func ObjectToUnstructured(obj interface{}) (unstr *unstructured.Unstructured, err error) {
	data, err := EncodeObject(obj)
	if err != nil {
		return unstr, err
	}
	unstrBody := map[string]interface{}{}
	err = json.Unmarshal(data, &unstrBody)
	return &unstructured.Unstructured{Object: unstrBody}, err
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

// GetEmailDomainFromEmail ...
// extract the domain from an email address
func GetEmailDomainFromEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at >= 0 {
		domain := email[at+1:]
		return domain
	}
	return ""
}

// AccountIsAdmin ...
// determine if account is an admin
func AccountIsAdmin(emails []types.GitHubEmail) bool {
	fmt.Println("EMAILS ::::", emails)
	adminEmailDomain := GetAdminEmailDomain()
	if adminEmailDomain == "" {
		fmt.Println("No admin orgs declared, assuming any account is admin")
		return true
	}
	for _, e := range emails {
		if GetEmailDomainFromEmail(e.Email) == adminEmailDomain {
			return true
		}
	}
	return false
}
