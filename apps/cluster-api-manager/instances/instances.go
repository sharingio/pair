package instances

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/asaskevich/govalidator"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/sharingio/pair/apps/cluster-api-manager/common"
)

// ValidateInstance ...
// ensure an Instance is valid
func ValidateInstance(instance InstanceSpec) (err error) {
	if common.ValidateName(instance.Name) == false && instance.Name != "" {
		return fmt.Errorf("Invalid instance name '%v'", instance.Name)
	}
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
		_, err = url.ParseRequestURI(repo)
		isURL := err == nil
		filePathValid, _ := govalidator.IsFilePath(repo)
		if repo == "" || !(isURL != true || filePathValid != true) {
			invalidRepos = append(invalidRepos, repo)
		}
	}
	if len(invalidRepos) > 0 {
		return fmt.Errorf("Invalid repos, %s", invalidRepos)
	}
	if govalidator.IsEmail(instance.Setup.Email) != true || instance.Setup.Email == "" {
		return fmt.Errorf("Invalid user email")
	}
	return nil
}

// Get ...
// get an instance
func Get(name string) (instance Instance, err error) {
	return instance, nil
}

// List ...
// list all instances
func List(dynamicClient dynamic.Interface, clientset *kubernetes.Clientset, options InstanceListOptions) (instances []Instance, err error) {
	switch options.Filter.Type {
	case InstanceTypeKubernetes:
		instances, err = KubernetesList(dynamicClient, clientset, options)
		break

	case InstanceTypePlain:
		break

	default:
		instances, err = KubernetesList(dynamicClient, clientset, options)
		// append plain type
	}
	return instances, nil
}

// Create ...
// create an instance
func Create(instance InstanceSpec, dynamicClient dynamic.Interface, clientset *kubernetes.Clientset, options InstanceCreateOptions) (instanceCreated InstanceSpec, err error) {
	err = ValidateInstance(instance)
	if err != nil {
		return instanceCreated, err
	}
	instancesOfUser, err := List(dynamicClient, clientset, InstanceListOptions{
		Filter: InstanceFilter{
			Username: instance.Setup.User,
		},
	})
	if err != nil {
		return instanceCreated, err
	}

	if common.AccountIsAdmin(instance.Setup.ExtraEmails) != true {
		switch len(instancesOfUser) {
		case common.GetNonAdminInstanceMaxAmount():
			return instanceCreated, fmt.Errorf("Max number of instances reached")
		}
	}

	instance.Setup.UserLowercase = strings.ToLower(instance.Setup.User)
	// uses instance.Name if specified
	// if no other instances exist
	if len(instancesOfUser) == 0 && instance.Name == "" {
		instance.Name = instance.Setup.UserLowercase
		options.NameScheme = InstanceNameSchemeSpecified
	} else if len(instancesOfUser) > 0 && instance.Name == "" {
		// if other instances exist
		instance.Name = GenerateName(instance)
		options.NameScheme = InstanceNameSchemeGenerateFromUsername
	} else if instance.Name != "" {
		options.NameScheme = InstanceNameSchemeSpecified
	}

	if options.NameScheme == InstanceNameSchemeSpecified {
		for _, existingInstance := range instancesOfUser {
			if instance.Name == existingInstance.Spec.Name {
				return instanceCreated, fmt.Errorf("An instance with the provided name already exists")
			}
		}
	}
	instance.Name = strings.ToLower(instance.Name)
	instance.NameScheme = options.NameScheme

	instance.Setup.Repos = common.AddRepoGitHubPrefix(instance.Setup.Repos)
	if instance.Setup.Timezone == "" {
		instance.Setup.Timezone = instanceDefaultTimezone
	}
	if instance.Setup.Fullname == "" {
		instance.Setup.Fullname = instance.Setup.User
	}
	switch instance.Type {
	case InstanceTypeKubernetes:
		instanceCreated, err = KubernetesCreate(instance, dynamicClient, clientset, options)
		break

	case InstanceTypePlain:
		break

	default:
		return InstanceSpec{}, fmt.Errorf("Invalid instance type")
	}
	return instanceCreated, nil
}

// Update ...
// update an instance
func Update(instance InstanceSpec) (instanceUpdated InstanceSpec, err error) {
	return instanceUpdated, nil
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
