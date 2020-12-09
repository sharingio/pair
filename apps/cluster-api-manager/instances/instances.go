package instances

import (
	"fmt"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/sharingio/pair/common"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// ValidateInstance ...
// ensure an Instance is valid
func ValidateInstance(instance InstanceSpec) (err error) {
	fmt.Println(instance)
	if common.ValidateName(instance.Name) == false {
		return fmt.Errorf("Invalid instance name '%v'", instance.Name)
	}
	if instance.Type == "" ||
		!(instance.Type == InstanceTypeKubernetes || instance.Type == InstanceTypePlain) {
		return fmt.Errorf("Invalid instance type")
	}
	if instance.Setup.User.Name == "" {
		return fmt.Errorf("No user declared")
	}
	if instance.Type == InstanceTypePlain {
		if len(instance.Setup.Guests) < 1 {
			return fmt.Errorf("No guests declared")
		}
		invalidGuests := []string{}
		for _, guest := range instance.Setup.Guests {
			if guest.Username == "" {
				invalidGuests = append(invalidGuests, guest.Username)
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
	if govalidator.IsEmail(instance.Setup.User.Email) != true || instance.Setup.User.Email == "" {
		return fmt.Errorf("Invalid user email")
	}
	if instance.Setup.User.Name == "" {
		return fmt.Errorf("Invalid name, name must not be empty")
	}
	return err
}

// Get ...
// get an instance
func Get(name string) (err error, instance Instance) {
	return err, instance
}

// List ...
// list all instances
func List(dynamicClient dynamic.Interface, options InstanceListOptions) (err error, instances []Instance) {
	switch options.Filter.Type {
	case InstanceTypeKubernetes:
		err, instances = KubernetesList(dynamicClient, options)
		break

	case InstanceTypePlain:
		break

	default:
		err, instances = KubernetesList(dynamicClient, options)
		// append plain type
	}
	return err, instances
}

// Create ...
// create an instance
func Create(instance Instance, dynamicClient dynamic.Interface, clientset *kubernetes.Clientset, options InstanceCreateOptions) (err error, instanceCreated InstanceSpec) {
	err, instancesOfUser := List(dynamicClient, InstanceListOptions{
		Filter: InstanceFilter{
			Username: instance.Spec.Setup.User.Username,
		},
	})
	if err != nil {
		return err, instanceCreated
	}
	instance.Spec.Setup.UserLowercase = strings.ToLower(instance.Spec.Setup.User.Username)
	// uses instance.Name if specified
	// if no other instances exist
	if len(instancesOfUser) == 0 && instance.Spec.Name == "" {
		instance.Spec.Name = instance.Spec.Setup.UserLowercase
		options.NameScheme = InstanceNameSchemeSpecified
	} else if len(instancesOfUser) > 0 && instance.Spec.Name == "" {
		// if other instances exist
		instance.Spec.Name = GenerateName(instance.Spec)
		options.NameScheme = InstanceNameSchemeGenerateFromUsername
	} else if instance.Spec.Name != "" {
		options.NameScheme = InstanceNameSchemeSpecified
	}

	if options.NameScheme == InstanceNameSchemeSpecified {
		for _, existingInstance := range instancesOfUser {
			if instance.Spec.Name == existingInstance.Spec.Name {
				return fmt.Errorf("An instance with the provided name already exists"), instanceCreated
			}
		}
	}
	instance.Spec.Name = strings.ToLower(instance.Spec.Name)

	instance.Spec.Setup.Repos = common.AddRepoGitHubPrefix(instance.Spec.Setup.Repos)
	if instance.Spec.Setup.Timezone == "" {
		instance.Spec.Setup.Timezone = instanceDefaultTimezone
	}
	err = ValidateInstance(instance.Spec)
	if err != nil {
		return err, instanceCreated
	}
	switch instance.Spec.Type {
	case InstanceTypeKubernetes:
		err, instanceCreated = KubernetesCreate(instance, dynamicClient, clientset, options)
		break

	case InstanceTypePlain:
		break

	default:
		return fmt.Errorf("Invalid instance type"), InstanceSpec{}
	}
	return err, instanceCreated
}

// Update ...
// update an instance
func Update(instance InstanceSpec) (err error, instanceUpdated InstanceSpec) {
	return err, instanceUpdated
}

// Delete ...
// delete an instance
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

