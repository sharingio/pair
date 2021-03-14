package instances

import (
	"fmt"
	"math/rand"
	"strings"
	"text/template"
	"time"

	"github.com/sharingio/pair/common"
)

// misc default vars
var (
	instanceDefaultNodeSize          = "c1.small.x86"
	instanceDefaultTimezone          = "Pacific/Auckland"
	instanceDefaultHumacsVersion     = "2021.03.09"
	instanceDefaultKubernetesVersion = "1.20.4"
)

// GetHumacsVersion ...
// get the version to deploy of the Humacs container
func GetHumacsVersion() string {
	return common.GetEnvOrDefault("APP_HUMACS_VERSION", instanceDefaultHumacsVersion)
}

// GetKubernetesVersion ...
// get the version of Kubernetes to use in the cluster
func GetKubernetesVersion() string {
	return common.GetEnvOrDefault("APP_INSTANCE_KUBERNETES_VERSION", instanceDefaultKubernetesVersion)
}

// GetInstanceDefaultNodeSize ...
// get the size of node to create
func GetInstanceDefaultNodeSize() string {
	return instanceDefaultNodeSize
}

// GenerateName ...
// given a username, append a 4 byte string to the end
func GenerateName(instance InstanceSpec) (name string) {
	rand.Seed(time.Now().UnixNano())
	randomString := common.RandomSequence(4)
	name = fmt.Sprintf("%s-%s", instance.Setup.User, randomString)
	name = strings.ToLower(name)

	return name
}

// TemplateFuncMap ...
// helpers for go templating
func TemplateFuncMap() template.FuncMap {
	return template.FuncMap{
		"add": func(n ...int) (output int) {
			for _, i := range n {
				output = output + i
			}
			fmt.Println("TemplateFuncMap add:", n, output)
			return output
		},
	}
}
