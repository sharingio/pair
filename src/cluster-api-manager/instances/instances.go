package instances

func Get(name string) (err error, instance InstanceSpec) {
	return err, instance
}

func List() (err string, instances []InstanceSpec) {
	return err, instances
}

func Create(instance InstanceSpec) (err error, instanceCreated InstanceSpec) {
	return err, instanceCreated
}

func Update(instance InstanceSpec) (err error, instanceUpdated InstanceSpec) {
	return err, instanceUpdated
}

func Delete(instance InstanceSpec) (err error, instanceDeleted InstanceSpec) {
	return err, instanceDeleted
}
