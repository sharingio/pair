package instances

import (
	"fmt"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/sharingio/pair/common"
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
	if govalidator.IsEmail(instance.Setup.Email) != true || instance.Setup.Email == "" {
		return fmt.Errorf("Invalid user email")
	}
	if instance.Setup.Fullname == "" {
		return fmt.Errorf("Invalid name, name must not be empty")
	}
	return err
}

func Get(name string) (err error, instance Instance) {
	return err, instance
}

func List(kubernetesClientset dynamic.Interface, options InstanceListOptions) (err string, instances []Instance) {
	return err, instances
}

func Create(instance InstanceSpec, kubernetesClientset dynamic.Interface, options InstanceCreateOptions) (err error, instanceCreated InstanceSpec) {
	err = ValidateInstance(instance)
	if err != nil {
		return err, instanceCreated
	}
	instance.Name = GenerateName(instance)
	instance.Setup.UserLowercase = strings.ToLower(instance.Setup.User)
	instance.Setup.Repos = common.AddRepoGitHubPrefix(instance.Setup.Repos)
	switch instance.Type {
	case InstanceTypeKubernetes:
		err, instanceCreated = KubernetesCreate(instance, kubernetesClientset, options)
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

