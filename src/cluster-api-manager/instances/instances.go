package instances

import (
	"crypto/md5"
	"fmt"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/sharingio/pair/src/cluster-api-manager/common"
	"k8s.io/client-go/dynamic"
)

func ValidateInstance(instance InstanceSpec) (err error) {
	fmt.Println(instance)
	if instance.Type == "" ||
		!(instance.Type == InstanceTypeKubernetes || instance.Type == InstanceTypePlain) {
		return fmt.Errorf("Invalid instance type")
	}
	if instance.Setup.User == "" {
		return fmt.Errorf("No user declared")
	}
	if instance.Type == InstanceTypePlain {
		if len(instance.Setup.Guests) < 1 {
			return fmt.Errorf("No guests declared")
		}
		invalidGuests := []string{}
		for _, guest := range instance.Setup.Guests {
			if guest == "" {
				invalidGuests = append(invalidGuests, guest)
			}
		}
		if len(invalidGuests) > 0 {
			return fmt.Errorf("Invalid guests, %s", invalidGuests)
		}
	}
	invalidRepos := []string{}
	for _, repo := range instance.Setup.Repos {
		filePathValid, _ := govalidator.IsFilePath(repo)
		if repo == "" || !(govalidator.IsURL(repo) != true || filePathValid != true) {
			invalidRepos = append(invalidRepos, repo)
		}
	}
	if len(invalidRepos) > 0 {
		return fmt.Errorf("Invalid repos, %s", invalidRepos)
	}
	return err
}

func Get(name string) (err error, instance Instance) {
	return err, instance
}

func List(kubernetesClientset dynamic.Interface, options InstanceListOptions) (err string, instances []Instance) {
	return err, instances
}

func Create(instance InstanceSpec, kubernetesClientset dynamic.Interface) (err error, instanceCreated InstanceSpec) {
	err = ValidateInstance(instance)
	if err != nil {
		return err, instanceCreated
	}
	instance.Name = GenerateName(instance)
	instance.Setup.Repos = common.AddRepoGitHubPrefix(instance.Setup.Repos)
	switch instance.Type {
	case InstanceTypeKubernetes:
		err, instanceCreated = KubernetesCreate(instance, kubernetesClientset)
		break

	case InstanceTypePlain:
		break

	default:
		return fmt.Errorf("Invalid instance type"), InstanceSpec{}
	}
	return err, instanceCreated
}

func Update(instance InstanceSpec) (err error, instanceUpdated InstanceSpec) {
	return err, instanceUpdated
}

func Delete(instance InstanceSpec, kubernetesClientset dynamic.Interface) (err error) {
	switch instance.Type {
	case InstanceTypeKubernetes:
		err = KubernetesDelete(instance.Name, kubernetesClientset)
		break

	case InstanceTypePlain:
		break

	default:
		return fmt.Errorf("Invalid instance type")
	}
	return err
}

func GenerateName(instance InstanceSpec) (name string) {
	name = fmt.Sprintf("%s", instance.Setup.User)
	guests := ""
	for _, guest := range instance.Setup.Guests {
		guests = fmt.Sprintf("%s", guest)
	}
	hashedString := md5.Sum([]byte(guests))
	name = fmt.Sprintf("%s-%x", name, hashedString[0:5])
	repos := ""
	for _, repo := range instance.Setup.Repos {
		repos = fmt.Sprintf("%s", repo)
	}
	hashedString = md5.Sum([]byte(repos))
	name = fmt.Sprintf("%s-%x", name, hashedString[0:5])
	name = strings.ToLower(name)

	return name
}
