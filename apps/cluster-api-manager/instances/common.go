package instances

import (
	"fmt"
	"math/rand"
	"strings"
	"text/template"
	"time"

	"github.com/sharingio/pair/apps/cluster-api-manager/common"
)

// misc default vars
var (
	instanceDefaultNodeSize              = "c3.small.x86"
	instanceDefaultTimezone              = "Pacific/Auckland"
	instanceDefaultEnvironmentRepository = "registry.gitlab.com/sharingio/environment/environment"
	instanceDefaultEnvironmentVersion    = "2022.03.30.1618"
	instanceDefaultKubernetesVersion     = "1.23.5"
)

// GetEnvironmentRepository ...
// get the container repository of where environment is
func GetEnvironmentRepository() string {
	return common.GetEnvOrDefault("APP_ENVIRONMENT_REPOSITORY", instanceDefaultEnvironmentRepository)
}

// GetEnvironmentVersion ...
// get the version to deploy of the Environment container
func GetEnvironmentVersion() string {
	return common.GetEnvOrDefault("APP_ENVIRONMENT_VERSION", instanceDefaultEnvironmentVersion)
}

// GetKubernetesVersion ...
// get the version of Kubernetes to use in the cluster
func GetKubernetesVersion() string {
	return common.GetEnvOrDefault("APP_INSTANCE_KUBERNETES_VERSION", instanceDefaultKubernetesVersion)
}

// GetInstanceDefaultNodeSize ...
// get the size of node to create
func GetInstanceDefaultNodeSize() string {
	return common.GetEnvOrDefault("APP_INSTANCE_NODE_SIZE", instanceDefaultNodeSize)
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
			return output
		},
	}
}

// GetValueFromEnvMap ...
// returns a value when keys of a map match
func GetValueFromEnvMap(input map[string]string, key string) string {
	for mapKey, value := range input {
		if mapKey == key {
			return value
		}
	}
	return ""
}

// GetValueFromEnvSlice ...
// returns a value when keys of a slice map match
func GetValueFromEnvSlice(input []map[string]string, key string) string {
	for _, sliceKey := range input {
		if value := GetValueFromEnvMap(sliceKey, key); value != "" {
			return value
		}
	}
	return ""
}
