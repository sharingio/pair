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
	"time"

	"github.com/asaskevich/govalidator"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/sharingio/pair/types"
)

var (
	AppBuildVersion = "0.0.0"
	AppBuildHash    = "???"
	AppBuildDate    = "???"
	AppBuildMode    = "development"
	letters         = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
)

func GetEnvOrDefault(envName string, defaultValue string) (output string) {
	output = os.Getenv(envName)
	if output == "" {
		output = defaultValue
	}
	return output
}

func GetAppPort() (output string) {
	return GetEnvOrDefault("APP_PORT", ":8080")
}

func GetPacketProjectID() (id string) {
	return GetEnvOrDefault("APP_PACKET_PROJECT_ID", "")
}

func GetTargetNamespace() (namespace string) {
	return GetEnvOrDefault("APP_TARGET_NAMESPACE", "sharingio-pair-instances")
}

func GetBaseHost() (host string) {
	return GetEnvOrDefault("APP_BASE_HOST", "")
}

func GetKubernetesSecretName() (secretName string) {
	return GetEnvOrDefault("APP_KUBERNETES_SECRET_NAME", "")
}

func Logging(next http.Handler) http.Handler {
	// log all requests
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%v %v %v %v %v", r.Method, r.URL, r.Proto, r.Response, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

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

func EncodeObject(obj interface{}) (err error, data []byte) {
	data, err = json.Marshal(obj)
	return err, data
}

func ObjectToUnstructured(obj interface{}) (err error, unstr *unstructured.Unstructured) {
	err, data := EncodeObject(obj)
	if err != nil {
		return err, unstr
	}
	unstrBody := map[string]interface{}{}
	err = json.Unmarshal(data, &unstrBody)
	return err, &unstructured.Unstructured{Object: unstrBody}
}

func AddRepoGitHubPrefix(repos []string) (reposModified []string) {
	for _, repo := range repos {
		if govalidator.IsURL(repo) != true {
			repo = fmt.Sprintf("https://github.com/%s", repo)
		}
		reposModified = append(reposModified, repo)
	}
	return reposModified
}

func ReverseStringArray(input []string) []string {
	output := make([]string, 0, len(input))
	for i := len(input) - 1; i >= 0; i-- {
		output = append(output, input[i])
	}
	return output
}

func RandomSequence(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
