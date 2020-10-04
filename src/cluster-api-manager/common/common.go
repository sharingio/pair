/*
	common function calls
*/

package common

import (
	"encoding/json"
	// corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	// "k8s.io/apimachinery/pkg/runtime"
	// "k8s.io/kubectl/pkg/scheme"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/sharingio/pair/src/cluster-api-manager/types"
)

var (
	AppVersion = "0.0.1"
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
	output.Metadata.Version = AppVersion
	response, _ := json.Marshal(output)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func EncodeObject(obj interface{}) (err error, data []byte) {
	// data, err = runtime.Encode(scheme.Codecs.LegacyCodec(corev1.SchemeGroupVersion), obj)
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
