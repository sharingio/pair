package instances

func Create(instance InstanceSpec) (err error, instanceCreated InstanceSpec) {}
func Update(instance InstanceSpec) (err error, instanceUpdated InstanceSpec) {}
func Delete(instance InstanceSpec) (err error, instanceDeleted InstanceSpec) {}
