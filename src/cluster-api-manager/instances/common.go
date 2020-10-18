package instances

import (
	"fmt"
	"crypto/md5"
	"strings"
)

const (
	instanceDefaultNodeSize = "c1.small.x86"
)

func GetInstanceDefaultNodeSize() (string) {
	return instanceDefaultNodeSize
}

func GenerateName(instance InstanceSpec) (name string) {
	name = fmt.Sprintf("%s", instance.Setup.User)
	portionOne := instance.Setup.Fullname + " " + instance.Setup.Email
	for _, guest := range instance.Setup.Guests {
		portionOne = fmt.Sprintf("%s", guest)
	}
	hashedString := md5.Sum([]byte(portionOne))
	name = fmt.Sprintf("%s-%x", name, hashedString[0:5])
	portionTwo := ""
	for _, repo := range instance.Setup.Repos {
		portionTwo = fmt.Sprintf("%s", repo)
	}
	hashedString = md5.Sum([]byte(portionTwo))
	name = fmt.Sprintf("%s-%x", name, hashedString[0:5])
	name = strings.ToLower(name)

	return name
}
