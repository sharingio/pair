package instances

import (
	"fmt"
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
		if repo == "" {
			invalidRepos = append(invalidRepos, repo)
		}
	}
	if len(invalidRepos) > 0 {
		return fmt.Errorf("Invalid repos, %s", invalidRepos)
	}
	return err
}

func Get(name string) (err error, instance InstanceSpec) {
	return err, instance
}

func List() (err string, instances []InstanceSpec) {
	return err, instances
}

func Create(instance InstanceSpec, kubernetesClientset dynamic.Interface) (err error, instanceCreated InstanceSpec) {
	err = ValidateInstance(instance)
	if err != nil {
		return err, instanceCreated
	}
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

func Delete(instance InstanceSpec) (err error) {
	return err
}
