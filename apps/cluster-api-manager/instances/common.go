package instances

import (
	"fmt"
	"strings"
	"math/rand"
	"time"

	"github.com/sharingio/pair/common"
)

const (
	instanceDefaultNodeSize = "c1.small.x86"
)

func GetInstanceDefaultNodeSize() (string) {
	return instanceDefaultNodeSize
}

func GenerateName(instance InstanceSpec) (name string) {
	rand.Seed(time.Now().UnixNano())
	randomString := common.RandomSequence(4)
	name = fmt.Sprintf("%s-%s", instance.Setup.User, randomString)
	name = strings.ToLower(name)

	return name
}
