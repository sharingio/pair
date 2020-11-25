package instances

import (
	"fmt"
	"math/rand"
	"strings"
	"text/template"
	"time"

	"github.com/sharingio/pair/common"
)

const (
	instanceDefaultNodeSize      = "c1.small.x86"
	instanceDefaultTimezone      = "Pacific/Auckland"
	instanceDefaultHumacsVersion = "2020.11.25"
)

func GetHumacsVersion() string {
	return common.GetEnvOrDefault("APP_HUMACS_VERSION", instanceDefaultHumacsVersion)
}

func GetInstanceDefaultNodeSize() string {
	return instanceDefaultNodeSize
}

func GenerateName(instance InstanceSpec) (name string) {
	rand.Seed(time.Now().UnixNano())
	randomString := common.RandomSequence(4)
	name = fmt.Sprintf("%s-%s", instance.Setup.User, randomString)
	name = strings.ToLower(name)

	return name
}

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
