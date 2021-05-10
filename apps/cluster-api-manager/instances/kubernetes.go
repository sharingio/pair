package instances

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"text/template"
	"time"

	"github.com/sharingio/pair/common"
	"github.com/sharingio/pair/dns"

	"github.com/asaskevich/govalidator"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/remotecommand"

	clusterAPIPacketv1alpha3 "sigs.k8s.io/cluster-api-provider-packet/api/v1alpha3"
	clusterAPIv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha3"
	kubeadmv1beta1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/types/v1beta1"
	clusterAPIControlPlaneKubeadmv1alpha3 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
	"sigs.k8s.io/yaml"
)

// KubernetesCluster ...
// resources required for Cluster-API to provision a Kubernetes cluster on Packet
type KubernetesCluster struct {
	KubeadmControlPlane         clusterAPIControlPlaneKubeadmv1alpha3.KubeadmControlPlane
	Cluster                     clusterAPIv1alpha3.Cluster
	MachineDeploymentWorker     clusterAPIv1alpha3.MachineDeployment
	KubeadmConfigTemplateWorker cabpkv1.KubeadmConfigTemplate
	PacketMachineTemplate       clusterAPIPacketv1alpha3.PacketMachineTemplate
	PacketCluster               clusterAPIPacketv1alpha3.PacketCluster
	PacketMachineTemplateWorker clusterAPIPacketv1alpha3.PacketMachineTemplate
}

// ExecOptions ...
// passed to ExecWithOptions
type ExecOptions struct {
	Command       []string
	Namespace     string
	PodName       string
	ContainerName string
	Stdin         io.Reader
	CaptureStdout bool
	CaptureStderr bool
	// If false, whitespace in std{err,out} will be removed.
	PreserveWhitespace bool
	TTY                bool
}

// Int32ToInt32Pointer ...
// helper function to make int32 into a pointer
func Int32ToInt32Pointer(input int32) *int32 {
	return &input
}

// misc vars
var (
	defaultMachineOS = "ubuntu_20_04"
)

// KubernetesGet ...
// Get a Kubernetes instance
func KubernetesGet(name string, kubernetesClientset dynamic.Interface) (err error, instance Instance) {
	targetNamespace := common.GetTargetNamespace()
	// manifests

	instance.Spec.Type = InstanceTypeKubernetes

	//   - newInstance.KubeadmControlPlane
	groupVersionResource := schema.GroupVersionResource{Version: "v1alpha3", Group: "controlplane.cluster.x-k8s.io", Resource: "kubeadmcontrolplanes"}
	log.Printf("%#v\n", groupVersionResource)
	item, err := kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Get(context.TODO(), fmt.Sprintf("%s-control-plane", name), metav1.GetOptions{})
	if err != nil {
		log.Printf("%#v\n", err)
	} else {
		var itemRestructuredKCP clusterAPIControlPlaneKubeadmv1alpha3.KubeadmControlPlane
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructuredKCP)
		if err != nil {
			return fmt.Errorf("Failed to restructure %T", itemRestructuredKCP), Instance{}
		}
		if itemRestructuredKCP.ObjectMeta.Labels["io.sharing.pair"] != "instance" {
			log.Printf("Not using object %s/%T/%s - not an instance managed by sharingio/pair\n", targetNamespace, itemRestructuredKCP, itemRestructuredKCP.ObjectMeta.Name)
		} else {
			instance.Status.Resources.KubeadmControlPlane = itemRestructuredKCP.Status
		}
	}

	//   - newInstance.Machine
	groupVersion := clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "cluster.x-k8s.io", Resource: "machines"}
	log.Printf("%#v\n", groupVersionResource)
	items, err := kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "cluster.x-k8s.io/cluster-name=" + name})
	if err != nil {
		log.Printf("%#v\n", err)
	} else {
		if len(items.Items) > 0 {
			item = &items.Items[0]
			var itemRestructuredM clusterAPIv1alpha3.Machine
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructuredM)
			if err != nil {
				return fmt.Errorf("Failed to restructure %T", itemRestructuredM), Instance{}
			}
			instance.Status.Resources.MachineStatus = itemRestructuredM.Status
		}
	}

	//   - newInstance.PacketMachine
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "infrastructure.cluster.x-k8s.io", Resource: "packetmachines"}
	log.Printf("%#v\n", groupVersionResource)
	items, err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "cluster.x-k8s.io/cluster-name=" + name})
	if err != nil {
		log.Printf("%#v\n", err)
	} else {
		if len(items.Items) > 0 {
			item = &items.Items[0]
			var itemRestructuredPM clusterAPIPacketv1alpha3.PacketMachine
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructuredPM)
			if err != nil {
				return fmt.Errorf("Failed to restructure %T", itemRestructuredPM), Instance{}
			}
			log.Printf("%#v\n", itemRestructuredPM.Spec)
			var providerID string = *itemRestructuredPM.Spec.ProviderID
			providerIDSplit := strings.Split(providerID, "/")
			if len(providerIDSplit) == 3 {
				instance.Status.Resources.PacketMachineUID = &providerIDSplit[2]
			}
		}
	}

	//   - newInstance.Cluster
	var itemRestructuredC clusterAPIv1alpha3.Cluster
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "cluster.x-k8s.io", Resource: "clusters"}
	log.Printf("%#v\n", groupVersionResource)
	item, err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to get Cluster, %#v", err), instance
	} else {
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructuredC)
		if err != nil {
			return fmt.Errorf("Failed to restructure %T", itemRestructuredC), Instance{}
		}
		if itemRestructuredC.ObjectMeta.Labels["io.sharing.pair"] != "instance" {
			log.Printf("Not using object %s/%T/%s - not an instance managed by sharingio/pair\n", targetNamespace, itemRestructuredC, itemRestructuredC.ObjectMeta.Name)
		} else {
			instance.Status.Resources.Cluster = itemRestructuredC.Status
		}
	}

	instance.Spec.Setup.User = itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-setup-user"]
	instance.Spec.Setup.UserLowercase = strings.ToLower(instance.Spec.Setup.User)

	err, kubeconfigBytes := KubernetesDynamicGetKubeconfigBytes(name, kubernetesClientset)
	if err != nil {
		log.Printf("%#v\n", err)
	}
	err, instanceClientset := KubernetesClientsetFromKubeconfigBytes(kubeconfigBytes)
	if err != nil {
		log.Printf("%#v\n", err)
	}
	deadline := time.Now().Add(time.Second * 2)
	ctx, _ := context.WithDeadline(context.TODO(), deadline)
	humacsPod, err := instanceClientset.CoreV1().Pods(instance.Spec.Setup.UserLowercase).Get(ctx, fmt.Sprintf("%s-humacs-0", instance.Spec.Setup.UserLowercase), metav1.GetOptions{})
	if err != nil {
		log.Printf("%#v\n", err)
	}
	instance.Status.Resources.HumacsPod = humacsPod.Status

	instance.Status.Phase = InstanceStatusPhaseProvisioning
	if instance.Status.Resources.Cluster.Phase == string(InstanceStatusPhaseDeleting) {
		instance.Status.Phase = InstanceStatusPhaseDeleting
	} else if instance.Status.Resources.HumacsPod.Phase == corev1.PodRunning {
		instance.Status.Phase = InstanceStatusPhaseProvisioned
	}

	instance.Spec.Name = itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-name"]
	instance.Spec.NodeSize = itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-nodeSize"]
	instance.Spec.Facility = itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-facility"]
	instance.Spec.Setup.Guests = strings.Split(itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-setup-guests"], " ")
	instance.Spec.Setup.Repos = strings.Split(itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-setup-repos"], " ")
	instance.Spec.Setup.Timezone = itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-setup-timezone"]
	instance.Spec.Setup.Fullname = itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-setup-fullname"]
	instance.Spec.Setup.Email = itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-setup-email"]
	var env []map[string]string
	json.Unmarshal([]byte(itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-setup-env"]), &env)
	instance.Spec.Setup.Env = env
	instance.Spec.Setup.BaseDNSName = itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-setup-baseDNSName"]

	err = nil
	return err, instance
}

// KubernetesList ...
// list all Kubernetes instances
func KubernetesList(kubernetesClientset dynamic.Interface, options InstanceListOptions) (err error, instances []Instance) {
	targetNamespace := common.GetTargetNamespace()

	// manifests

	groupVersionResource := schema.GroupVersionResource{Version: "v1alpha3", Group: "controlplane.cluster.x-k8s.io", Resource: "kubeadmcontrolplanes"}
	log.Printf("%#v\n", groupVersionResource)
	items, err := kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to list KubeadmControlPlane, %#v", err), instances
	}

	for _, item := range items.Items {
		var itemRestructured clusterAPIControlPlaneKubeadmv1alpha3.KubeadmControlPlane
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructured)
		if err != nil {
			return fmt.Errorf("Failed to restructure %T", itemRestructured), []Instance{}
		}
		if itemRestructured.ObjectMeta.Labels["io.sharing.pair"] != "instance" {
			log.Printf("Not using object %s/%T/%s - not an instance managed by sharingio/pair\n", targetNamespace, itemRestructured, itemRestructured.ObjectMeta.Name)
			continue
		}
		if options.Filter.Username != "" && itemRestructured.ObjectMeta.Annotations["io.sharing.pair-spec-setup-user"] != options.Filter.Username {
			log.Printf("Not using object %s/%T/%s - not related to username\n", targetNamespace, itemRestructured, itemRestructured.ObjectMeta.Name)
			continue
		}
		var instance = Instance{
			Spec: InstanceSpec{
				Name: itemRestructured.ObjectMeta.Annotations["io.sharing.pair-spec-name"],
			},
			Status: InstanceStatus{
				Resources: InstanceResourceStatus{
					KubeadmControlPlane: itemRestructured.Status,
				},
			},
		}
		instances = append(instances, instance)
	}

	//   - newInstance.Machine
	groupVersion := clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "cluster.x-k8s.io", Resource: "machines"}
	log.Printf("%#v\n", groupVersionResource)
	items, err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to list Machine, %#v", err), instances
	}

	for _, item := range items.Items {
		var itemRestructured clusterAPIv1alpha3.Machine
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructured)
		if err != nil {
			return fmt.Errorf("Failed to restructure %T", itemRestructured), []Instance{}
		}
	instances1:
		for i := range instances {
			if instances[i].Spec.Name == itemRestructured.ObjectMeta.Labels["cluster.x-k8s.io/cluster-name"] {
				instances[i].Status.Resources.MachineStatus = itemRestructured.Status
				break instances1
			}
		}
	}

	//   - newInstance.PacketMachine
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "infrastructure.cluster.x-k8s.io", Resource: "packetmachines"}
	log.Printf("%#v\n", groupVersionResource)
	items, err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to list PacketMachine, %#v", err), instances
	}

	for _, item := range items.Items {
		var itemRestructured clusterAPIPacketv1alpha3.PacketMachine
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructured)
		if err != nil {
			return fmt.Errorf("Failed to restructure %T", itemRestructured), []Instance{}
		}
	instances2:
		for i := range instances {
			if instances[i].Spec.Name == itemRestructured.ObjectMeta.Labels["cluster.x-k8s.io/cluster-name"] {
				if itemRestructured.Spec.ProviderID == nil {
					continue instances2
				}
				var providerID string = *itemRestructured.Spec.ProviderID
				providerIDSplit := strings.Split(providerID, "/")
				if len(providerIDSplit) == 3 {
					instances[i].Status.Resources.PacketMachineUID = &providerIDSplit[2]
				}
				break instances2
			}
		}
	}

	//   - newInstance.Cluster
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "cluster.x-k8s.io", Resource: "clusters"}
	log.Printf("%#v\n", groupVersionResource)
	items, err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to list Cluster, %#v", err), instances
	}

	for _, item := range items.Items {
		var itemRestructured clusterAPIv1alpha3.Cluster
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructured)
		if err != nil {
			return fmt.Errorf("Failed to restructure %T", itemRestructured), []Instance{}
		}
		if itemRestructured.ObjectMeta.Labels["io.sharing.pair"] != "instance" {
			log.Printf("Not using object %s/%T/%s - not an instance managed by sharingio/pair\n", targetNamespace, itemRestructured, itemRestructured.ObjectMeta.Name)
			continue
		}
		if options.Filter.Username != "" && itemRestructured.ObjectMeta.Annotations["io.sharing.pair-spec-setup-user"] != options.Filter.Username {
			log.Printf("Not using object %s/%T/%s - not related to username\n", targetNamespace, itemRestructured, itemRestructured.ObjectMeta.Name)
			continue
		}
	instances3:
		for i := range instances {
			if instances[i].Spec.Name == itemRestructured.ObjectMeta.Annotations["io.sharing.pair-spec-name"] {
				instances[i].Spec.Type = InstanceTypeKubernetes
				instances[i].Status.Resources.Cluster = itemRestructured.Status
				instances[i].Spec.Name = itemRestructured.ObjectMeta.Annotations["io.sharing.pair-spec-name"]
				instances[i].Spec.NodeSize = itemRestructured.ObjectMeta.Annotations["io.sharing.pair-spec-nodeSize"]
				instances[i].Spec.Facility = itemRestructured.ObjectMeta.Annotations["io.sharing.pair-spec-facility"]
				instances[i].Spec.Setup.User = itemRestructured.ObjectMeta.Annotations["io.sharing.pair-spec-setup-user"]
				instances[i].Spec.Setup.Guests = strings.Split(itemRestructured.ObjectMeta.Annotations["io.sharing.pair-spec-setup-guests"], " ")
				instances[i].Spec.Setup.Repos = strings.Split(itemRestructured.ObjectMeta.Annotations["io.sharing.pair-spec-setup-repos"], " ")
				instances[i].Spec.Setup.Timezone = itemRestructured.ObjectMeta.Annotations["io.sharing.pair-spec-setup-timezone"]
				instances[i].Spec.Setup.Fullname = itemRestructured.ObjectMeta.Annotations["io.sharing.pair-spec-setup-fullname"]
				instances[i].Spec.Setup.Email = itemRestructured.ObjectMeta.Annotations["io.sharing.pair-spec-setup-email"]
				var env []map[string]string
				json.Unmarshal([]byte(itemRestructured.ObjectMeta.Annotations["io.sharing.pair-spec-setup-env"]), &env)
				instances[i].Spec.Setup.Env = env
				instances[i].Status.Resources.Cluster = itemRestructured.Status

				instances[i].Status.Phase = InstanceStatusPhaseProvisioning
				if instances[i].Status.Resources.Cluster.Phase == string(InstanceStatusPhaseDeleting) {
					instances[i].Status.Phase = InstanceStatusPhaseDeleting
				} else if instances[i].Status.Resources.HumacsPod.Phase == corev1.PodRunning {
					instances[i].Status.Phase = InstanceStatusPhaseProvisioned
				}
				break instances3
			}
		}
	}
	err = nil
	return err, instances
}

// KubernetesCreate ...
// create a Kubernetes Instance
func KubernetesCreate(instance InstanceSpec, dynamicClient dynamic.Interface, clientset *kubernetes.Clientset, options InstanceCreateOptions) (err error, instanceCreated InstanceSpec) {
	// generate name
	targetNamespace := common.GetTargetNamespace()
	err, newInstance := KubernetesTemplateResources(instance, targetNamespace)
	if err != nil {
		return err, instanceCreated
	}
	instanceCreated = instance

	log.Printf("%#v\n", newInstance)

	if options.DryRun == true {
		log.Println("Exiting before create due to dry run")
		postKubeadmCommandYAML, _ := yaml.Marshal(newInstance.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands)
		log.Printf("%v\n", string(postKubeadmCommandYAML))
		return err, instanceCreated
	}

	// manifests
	//   - newInstance.KubeadmControlPlane
	groupVersionResource := schema.GroupVersionResource{Version: "v1alpha3", Group: "controlplane.cluster.x-k8s.io", Resource: "kubeadmcontrolplanes"}
	log.Printf("%#v\n", groupVersionResource)
	err, asUnstructured := common.ObjectToUnstructured(newInstance.KubeadmControlPlane)
	asUnstructured.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1alpha3", Group: "controlplane.cluster.x-k8s.io", Kind: "KubeadmControlPlane"})
	_, err = dynamicClient.Resource(groupVersionResource).Namespace(targetNamespace).Create(context.TODO(), asUnstructured, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create KubeadmControlPlane, %#v", err), instanceCreated
	}
	if apierrors.IsAlreadyExists(err) {
		log.Println("Already exists")
	}

	//   - newInstance.PacketMachineTemplate
	groupVersion := clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "infrastructure.cluster.x-k8s.io", Resource: "packetmachinetemplates"}
	log.Printf("%#v\n", groupVersionResource)
	err, asUnstructured = common.ObjectToUnstructured(newInstance.PacketMachineTemplate)
	asUnstructured.SetGroupVersionKind(schema.GroupVersionKind{Version: groupVersionResource.Version, Group: "infrastructure.cluster.x-k8s.io", Kind: "PacketMachineTemplate"})
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to unstructure PacketMachineTemplate, %#v", err), instanceCreated
	}
	_, err = dynamicClient.Resource(groupVersionResource).Namespace(targetNamespace).Create(context.TODO(), asUnstructured, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create PacketMachineTemplate, %#v", err), instanceCreated
	}
	if apierrors.IsAlreadyExists(err) {
		log.Println("Already exists")
	}

	//   - newInstance.PacketCluster
	groupVersion = clusterAPIPacketv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "infrastructure.cluster.x-k8s.io", Resource: "packetclusters"}
	log.Printf("%#v\n", groupVersionResource)
	err, asUnstructured = common.ObjectToUnstructured(newInstance.PacketCluster)
	asUnstructured.SetGroupVersionKind(schema.GroupVersionKind{Version: groupVersionResource.Version, Group: "infrastructure.cluster.x-k8s.io", Kind: "PacketCluster"})
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to unstructure PacketCluster, %#v", err), instanceCreated
	}
	_, err = dynamicClient.Resource(groupVersionResource).Namespace(targetNamespace).Create(context.TODO(), asUnstructured, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create PacketCluster, %#v", err), instanceCreated
	}
	if apierrors.IsAlreadyExists(err) {
		log.Println("Already exists")
	}

	//   - newInstance.Cluster
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "cluster.x-k8s.io", Resource: "clusters"}
	log.Printf("%#v\n", groupVersionResource)
	err, asUnstructured = common.ObjectToUnstructured(newInstance.Cluster)
	asUnstructured.SetGroupVersionKind(schema.GroupVersionKind{Version: groupVersionResource.Version, Group: "cluster.x-k8s.io", Kind: "Cluster"})
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to unstructure Cluster, %#v", err), instanceCreated
	}
	_, err = dynamicClient.Resource(groupVersionResource).Namespace(targetNamespace).Create(context.TODO(), asUnstructured, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create Cluster, %#v", err), instanceCreated
	}
	if apierrors.IsAlreadyExists(err) {
		log.Println("Already exists")
	}

	//   - newInstance.MachineDeploymentWorker
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "cluster.x-k8s.io", Resource: "machinedeployments"}
	log.Printf("%#v\n", groupVersionResource)
	err, asUnstructured = common.ObjectToUnstructured(newInstance.MachineDeploymentWorker)
	asUnstructured.SetGroupVersionKind(schema.GroupVersionKind{Version: groupVersionResource.Version, Group: "cluster.x-k8s.io", Kind: "MachineDeployment"})
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to unstructure MachineDeployment, %#v", err), instanceCreated
	}
	_, err = dynamicClient.Resource(groupVersionResource).Namespace(targetNamespace).Create(context.TODO(), asUnstructured, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create MachineDeployment, %#v", err), instanceCreated
	}
	if apierrors.IsAlreadyExists(err) {
		log.Println("Already exists")
	}

	//   - newInstance.KubeadmConfigTemplateWorker
	groupVersion = cabpkv1.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "bootstrap.cluster.x-k8s.io", Resource: "kubeadmconfigtemplates"}
	log.Printf("%#v\n", groupVersionResource)
	err, asUnstructured = common.ObjectToUnstructured(newInstance.KubeadmConfigTemplateWorker)
	asUnstructured.SetGroupVersionKind(schema.GroupVersionKind{Version: groupVersionResource.Version, Group: groupVersionResource.Group, Kind: "KubeadmConfigTemplate"})
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to unstructure KubeadmConfigTemplate, %#v", err), instanceCreated
	}
	_, err = dynamicClient.Resource(groupVersionResource).Namespace(targetNamespace).Create(context.TODO(), asUnstructured, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create KubeadmConfigTemplate, %#v", err), instanceCreated
	}
	if apierrors.IsAlreadyExists(err) {
		log.Println("Already exists")
	}

	//   - newInstance.PacketMachineTemplateWorker
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "infrastructure.cluster.x-k8s.io", Resource: "packetmachinetemplates"}
	log.Printf("%#v\n", groupVersionResource)
	err, asUnstructured = common.ObjectToUnstructured(newInstance.PacketMachineTemplateWorker)
	asUnstructured.SetGroupVersionKind(schema.GroupVersionKind{Version: groupVersionResource.Version, Group: "infrastructure.cluster.x-k8s.io", Kind: "PacketMachineTemplate"})
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to unstructure PacketMachineTemplateWorker, %#v", err), instanceCreated
	}
	_, err = dynamicClient.Resource(groupVersionResource).Namespace(targetNamespace).Create(context.TODO(), asUnstructured, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to create PacketMachineTemplateWorker, %#v", err), instanceCreated
	}
	if apierrors.IsAlreadyExists(err) {
		log.Println("Already exists")
	}
	log.Println("[pre] adding Kubernetes Instance IP to DNS")

	go KubernetesAddMachineIPToDNS(dynamicClient, instance.Name, instance.Name)
	if options.NameScheme == InstanceNameSchemeSpecified || options.NameScheme == InstanceNameSchemeUsername {
		go KubernetesAddCertToMachine(clientset, dynamicClient, instance.Name)
	}
	log.Println("[post] adding Kubernetes Instance IP to DNS")

	err = nil

	// TODO return the same creation fields (repos, guests, etc...)
	return err, instanceCreated
}

// KubernetesUpdate ...
// update a Kubernetes instance
func KubernetesUpdate(instance InstanceSpec) (err error, instanceUpdated InstanceSpec) {
	return err, instanceUpdated
}

// KubernetesDelete ...
// delete a Kubernetes instance
func KubernetesDelete(name string, kubernetesClientset dynamic.Interface) (err error) {
	// generate name
	targetNamespace := common.GetTargetNamespace()

	// manifests

	//   - newInstance.PacketMachineTemplate
	groupVersion := clusterAPIv1alpha3.GroupVersion
	groupVersionResource := schema.GroupVersionResource{Version: groupVersion.Version, Group: "infrastructure.cluster.x-k8s.io", Resource: "packetmachinetemplates"}
	log.Printf("%#v\n", groupVersionResource)
	err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to delete PacketMachineTemplate, %#v", err)
	}

	//   - newInstance.KubeadmConfigTemplateWorker
	groupVersion = cabpkv1.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "bootstrap.cluster.x-k8s.io", Resource: "kubeadmconfigtemplates"}
	log.Printf("%#v\n", groupVersionResource)
	err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Delete(context.TODO(), fmt.Sprintf("%s-worker-a", name), metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to delete KubeadmConfigTemplate, %#v", err)
	}

	//   - newInstance.PacketMachineTemplateWorker
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "infrastructure.cluster.x-k8s.io", Resource: "packetmachinetemplates"}
	log.Printf("%#v\n", groupVersionResource)
	err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Delete(context.TODO(), fmt.Sprintf("%s-worker-a", name), metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to delete PacketMachineTemplateWorker, %#v", err)
	}

	//   - newInstance.PacketMachine
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "infrastructure.cluster.x-k8s.io", Resource: "packetmachines"}
	log.Printf("%#v\n", groupVersionResource)
	err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: "cluster.x-k8s.io/cluster-name=" + name})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to delete PacketMachine, %#v", err)
	}

	//   - newInstance.Machine
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "cluster.x-k8s.io", Resource: "machines"}
	log.Printf("%#v\n", groupVersionResource)
	err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: "cluster.x-k8s.io/cluster-name=" + name})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to delete PacketMachine, %#v", err)
	}

	//   - newInstance.Cluster
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "cluster.x-k8s.io", Resource: "clusters"}
	log.Printf("%#v\n", groupVersionResource)
	err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to delete Cluster, %#v", err)
	}
	err = nil

	return err
}

// KubernetesTemplateResources ...
// given an instance spec and namespace, return KubernetesCluster resources
func KubernetesTemplateResources(instance InstanceSpec, namespace string) (err error, newInstance KubernetesCluster) {
	instance.NodeSize = common.ReturnValueOrDefault(instance.NodeSize, GetInstanceDefaultNodeSize())
	instance.Setup.HumacsVersion = common.ReturnValueOrDefault(instance.Setup.HumacsVersion, GetHumacsVersion())
	instance.Setup.HumacsRepository = common.ReturnValueOrDefault(instance.Setup.HumacsRepository, GetHumacsRepository())
	instance.Setup.KubernetesVersion = common.ReturnValueOrDefault(instance.Setup.KubernetesVersion, GetKubernetesVersion())
	instance = UpdateInstanceSpecIfEnvOverrides(instance)

	defaultKubernetesClusterConfig := KubernetesCluster{
		KubeadmControlPlane: clusterAPIControlPlaneKubeadmv1alpha3.KubeadmControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "",
				Labels: map[string]string{"io.sharing.pair": "instance"},
			},
			Spec: clusterAPIControlPlaneKubeadmv1alpha3.KubeadmControlPlaneSpec{
				Version:  instance.Setup.KubernetesVersion,
				Replicas: Int32ToInt32Pointer(1),
				InfrastructureTemplate: corev1.ObjectReference{
					APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha3",
					Kind:       "PacketMachineTemplate",
				},
				KubeadmConfigSpec: cabpkv1.KubeadmConfigSpec{
					InitConfiguration: &kubeadmv1beta1.InitConfiguration{
						NodeRegistration: kubeadmv1beta1.NodeRegistrationOptions{
							KubeletExtraArgs: map[string]string{
								"cloud-provider": "external",
							},
						},
					},
					ClusterConfiguration: &kubeadmv1beta1.ClusterConfiguration{
						APIServer: kubeadmv1beta1.APIServer{
							ControlPlaneComponent: kubeadmv1beta1.ControlPlaneComponent{
								ExtraArgs: map[string]string{
									"cloud-provider":            "external",
									"audit-policy-file":         "/etc/kubernetes/pki/audit-policy.yaml",
									"audit-log-path":            "-",
									"audit-webhook-config-file": "/etc/kubernetes/pki/audit-sink.yaml",
									"v":                         "99",
								},
							},
						},
						ControllerManager: kubeadmv1beta1.ControlPlaneComponent{
							ExtraArgs: map[string]string{
								"cloud-provider": "external",
							},
						},
					},
					JoinConfiguration: &kubeadmv1beta1.JoinConfiguration{
						NodeRegistration: kubeadmv1beta1.NodeRegistrationOptions{
							KubeletExtraArgs: map[string]string{
								"cloud-provider": "external",
							},
						},
					},
					PreKubeadmCommands: []string{
						"mkdir -p /etc/kubernetes/pki",
						`cat <<EOF > /etc/kubernetes/pki/audit-policy.yaml
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
- level: RequestResponse
EOF`,
						`cat <<EOF > /etc/kubernetes/pki/audit-sink.yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: http://10.96.96.96:9900/events
  name: auditsink-cluster
contexts:
- context:
    cluster: auditsink-cluster
    user: ""
  name: auditsink-context
current-context: auditsink-context
users: []
preferences: {}
EOF`,
						"sed -ri '/\\sswap\\s/s/^#?/#/' /etc/fstab",
						"swapoff -a",
						"mount -a",
						"apt-get -y update",
						"DEBIAN_FRONTEND=noninteractive apt-get install -y apt-transport-https curl",
						"curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key add -",
						"echo \"deb https://apt.kubernetes.io/ kubernetes-xenial main\" > /etc/apt/sources.list.d/kubernetes.list",
						"curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -",
						"apt-key fingerprint 0EBFCD88",
						"add-apt-repository \"deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable\"",
						"apt-get update -y",
						fmt.Sprintf(`TRIMMED_KUBERNETES_VERSION=$(echo %s | sed 's/\./\\./g' | sed 's/^v//')`, instance.Setup.KubernetesVersion),
						"RESOLVED_KUBERNETES_VERSION=$(apt-cache policy kubelet | awk -v VERSION=${TRIMMED_KUBERNETES_VERSION} '$1~ VERSION { print $1 }' | head -n1)",
						"apt-get install -y ca-certificates socat jq ebtables apt-transport-https cloud-utils prips docker-ce docker-ce-cli containerd.io kubelet=${RESOLVED_KUBERNETES_VERSION} kubeadm=${RESOLVED_KUBERNETES_VERSION} kubectl=${RESOLVED_KUBERNETES_VERSION} ssh-import-id dnsutils kitty-terminfo git",
						`cat <<EOF | tee /etc/modules-load.d/containerd.conf
overlay
br_netfilter
EOF
modprobe overlay
modprobe br_netfilter
cat <<EOF | tee /etc/sysctl.d/99-kubernetes-cri.conf
net.bridge.bridge-nf-call-iptables  = 1
net.ipv4.ip_forward                 = 1
net.bridge.bridge-nf-call-ip6tables = 1
EOF
sysctl --system
`,
						"mkdir -p /etc/containerd",
						"rm /etc/containerd/config.toml",
						"systemctl restart containerd",
						"export CONTAINER_RUNTIME_ENDPOINT=/var/run/containerd/containerd.sock",
						"echo $HOME",
						"export HOME=$(getent passwd $(id -u) | cut -d ':' -f6)",
						`cat <<EOF > /etc/docker/daemon.json
{
  "storage-driver": "overlay2",
  "exec-opts": ["native.cgroupdriver=systemd"],
  "containerd-namespace": "k8s.io",
  "containerd-plugins-namespace": "k8s.io",
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "500m",
    "max-file": "3"
  }
}
EOF`,
						"systemctl daemon-reload",
						"systemctl enable docker",
						"systemctl start docker",
						`
until systemctl status docker; do
  echo "Docker not ready"
  sleep 1s
done
`,
						"chgrp users /var/run/docker.sock",
						"ping -c 3 -q {{ .controlPlaneEndpoint }} && echo OK || ip addr add {{ .controlPlaneEndpoint }} dev lo",
					},
					PostKubeadmCommands: []string{
						`set -x`,
						`cat <<EOF >> /etc/network/interfaces
auto lo:0
iface lo:0 inet static
  address {{ .controlPlaneEndpoint }}
  netmask 255.255.255.255
EOF
`,
						"mkdir -p /root/.kube",
						"cp -i /etc/kubernetes/admin.conf /root/.kube/config",
						"export KUBECONFIG=/root/.kube/config",
						`
mkdir -p /etc/sudoers.d
echo "%sudo    ALL=(ALL:ALL) NOPASSWD: ALL" > /etc/sudoers.d/sudo
cp -a /root/.ssh /etc/skel/.ssh
useradd -m -G users,sudo -u 1000 -s /bin/bash ii
cp -a /root/.kube /home/ii/.kube
chown ii:ii -R /home/ii/.kube
`,
						`
sudo -iu ii ssh-import-id gh:{{ $.Setup.User }}
{{ range $.Setup.Guests }}
sudo -iu ii ssh-import-id gh:{{ . }}
{{ end }}
`,
						"kubectl -n default get configmap sharingio-pair-init-complete && exit 0",
						"kubectl taint node --all node-role.kubernetes.io/master-",
						"kubectl create secret generic -n kube-system packet-cloud-config --from-literal=cloud-sa.json='{\"apiKey\": \"{{ .apiKey }}\",\"projectID\": \"{{ .PacketProjectID }}\"}'",
						"kubectl apply -f https://github.com/packethost/packet-ccm/releases/download/v2.0.0/deployment.yaml",
						"kubectl taint node --all node-role.kubernetes.io/master-",
						"kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.19/deploy/local-path-storage.yaml",
						`kubectl patch storageclasses.storage.k8s.io local-path -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'`,
						"kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.1.1/cert-manager.yaml",
						"kubectl apply -f \"https://cloud.weave.works/k8s/net?k8s-version=$(kubectl version | base64 | tr -d '\n')&env.IPALLOC_RANGE=192.168.0.0/16\"",
						"curl -L https://get.helm.sh/helm-v3.3.0-linux-amd64.tar.gz | tar --directory /usr/local/bin --extract -xz --strip-components 1 linux-amd64/helm",
						`
helm repo add fluxcd https://charts.fluxcd.io
kubectl apply -f https://raw.githubusercontent.com/fluxcd/helm-operator/1.2.0/deploy/crds.yaml
helm upgrade -i helm-operator --create-namespace fluxcd/helm-operator \
    --namespace helm-operator \
    --set helm.versions=v3

kubectl apply -f https://raw.githubusercontent.com/alexellis/registry-creds/0.2.5/manifest.yaml
`,
						"kubectl get configmap kube-proxy -n kube-system -o yaml | sed -e \"s/strictARP: false/strictARP: true/\" | kubectl apply -f - -n kube-system",
						`cat <<EOF > /root/metallb-system-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: metallb-system
  name: config
data:
  config: |
    address-pools:
      - name: default
        protocol: layer2
        addresses:
          - {{ .controlPlaneEndpoint }}/32
EOF
export LOAD_BALANCER_IP="{{ .controlPlaneEndpoint }}"
`,
						`(
          kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.9.3/manifests/namespace.yaml;
          kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.9.3/manifests/metallb.yaml;
          kubectl create secret generic -n metallb-system memberlist --from-literal=secretkey="$(openssl rand -base64 128)";
          kubectl apply -f /root/metallb-system-config.yaml
        )
`,
						`
kubectl create ns nginx-ingress
cat << EOF | envsubst | kubectl apply -f -
apiVersion: helm.fluxcd.io/v1
kind: HelmRelease
metadata:
  name: nginx-ingress
  namespace: nginx-ingress
spec:
  releaseName: nginx-ingress
  chart:
    repository: https://kubernetes.github.io/ingress-nginx
    name: ingress-nginx
    version: 2.16.0
  values:
    controller:
      service:
        externalTrafficPolicy: Local
        annotations:
          metallb.universe.tf/allow-shared-ip: nginx-ingress
        externalIPs:
          - ${LOAD_BALANCER_IP}
EOF
until kubectl -n nginx-ingress get deployment nginx-ingress-ingress-nginx-controller; do
  echo "waiting for nginx-ingress deployment"
  sleep 5s;
done
kubectl wait -n nginx-ingress --for=condition=ready pod --selector=app.kubernetes.io/component=controller --timeout=90s

cat << EOF | kubectl apply -f -
apiVersion: helm.fluxcd.io/v1
kind: HelmRelease
metadata:
  name: metrics-server
  namespace: kube-system
spec:
  releaseName: metrics-server
  chart:
    repository: https://charts.helm.sh/stable
    name: metrics-server
    version: 2.11.2
  values:
    args:
      - --logtostderr
      - --kubelet-preferred-address-types=InternalIP
      - --kubelet-insecure-tls
EOF
`,
						`(
  kubectl create ns external-dns
  kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/external-dns/v0.8.0/docs/contributing/crd-source/crd-manifest.yaml
  kubectl -n external-dns create secret generic external-dns-pdns \
    --from-literal=domain-filter={{ $.Setup.BaseDNSName }} \
    --from-literal=txt-owner-id={{ $.Setup.User }} \
    --from-literal=pdns-server=http://powerdns-service-api.powerdns:8081 \
    --from-literal=pdns-api-key=pairingissharing

  kubectl -n external-dns apply -f - << EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: external-dns
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: external-dns
rules:
- apiGroups:
    - ""
  resources:
    - services
    - endpoints
    - pods
  verbs:
    - get
    - watch
    - list
- apiGroups:
    - extensions
    - networking.k8s.io
  resources:
    - ingresses
  verbs:
    - get
    - watch
    - list
- apiGroups:
    - externaldns.k8s.io
  resources:
    - dnsendpoints
  verbs:
    - get
    - watch
    - list
- apiGroups:
    - externaldns.k8s.io
  resources:
    - dnsendpoints/status
  verbs:
  - get
  - update
  - patch
  - delete
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: external-dns-viewer
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: external-dns
subjects:
- kind: ServiceAccount
  name: external-dns
  namespace: external-dns
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: external-dns
spec:
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: external-dns
  template:
    metadata:
      labels:
        app: external-dns
    spec:
      serviceAccountName: external-dns
      containers:
      - name: external-dns
        image: k8s.gcr.io/external-dns/external-dns:v0.8.0
        args:
        - --source=crd
        - --crd-source-apiversion=externaldns.k8s.io/v1alpha1
        - --crd-source-kind=DNSEndpoint
        - --provider=pdns
        - --policy=sync
        - --registry=txt
        - --interval=10s
        - --log-level=debug
        env:
          - name: EXTERNAL_DNS_DOMAIN_FILTER
            valueFrom:
              secretKeyRef:
                name: external-dns-pdns
                key: domain-filter
          - name: EXTERNAL_DNS_TXT_OWNER_ID
            valueFrom:
              secretKeyRef:
                name: external-dns-pdns
                key: txt-owner-id
          - name: EXTERNAL_DNS_PDNS_SERVER
            valueFrom:
              secretKeyRef:
                name: external-dns-pdns
                key: pdns-server
          - name: EXTERNAL_DNS_PDNS_API_KEY
            valueFrom:
              secretKeyRef:
                name: external-dns-pdns
                key: pdns-api-key
          - name: EXTERNAL_DNS_PDNS_TLS_ENABLED
            value: "0"
EOF
)`,
						`
kubectl create ns powerdns
cat << EOF | envsubst | kubectl apply -f -
apiVersion: helm.fluxcd.io/v1
kind: HelmRelease
metadata:
  name: powerdns
  namespace: powerdns
spec:
  releaseName: powerdns
  chart:
    name: powerdns
    repository: https://raw.githubusercontent.com/sharingio/helm-charts/gh-pages/
    version: 0.1.12
  values:
    domain: {{ $.Setup.BaseDNSName }}
    default_soa_name: {{ $.Setup.BaseDNSName }}
    powerdns:
      default_ttl: 60
      soa_minimum_ttl: 60
      domain: {{ $.Setup.BaseDNSName }}
      mysql_host: powerdns-service-db
      mysql_user: powerdns
      extraEnv:
        - name: PDNS_dnsupdate
          value: "yes"
        - name: PDNS_allow_dnsupdate_from
          value: "192.168.0.0/24"
    service:
      dns:
        tcp:
          enabled: true
          externalIPs:
            - ${LOAD_BALANCER_IP}
          annotations:
            metallb.universe.tf/allow-shared-ip: nginx-ingress
        udp:
          externalIPs:
            - ${LOAD_BALANCER_IP}
          annotations:
            metallb.universe.tf/allow-shared-ip: nginx-ingress
    mariadb:
      mysql_pass: pairingissharing
      mysql_rootpass: pairingissharing
    admin:
      enabled: false
      ingress:
        enabled: false
      secret: pairingissharing
    apikey: pairingissharing
EOF

until kubectl -n powerdns get svc powerdns-service-dns-udp; do
  echo "waiting for deployed PowerDNS Chart"
  sleep 5s
done
kubectl -n powerdns patch svc powerdns-service-dns-udp -p "{\"spec\":{\"externalIPs\":[\"${LOAD_BALANCER_IP}\"]}}"
kubectl -n powerdns patch svc powerdns-service-dns-tcp -p "{\"spec\":{\"externalIPs\":[\"${LOAD_BALANCER_IP}\"]}}"

  kubectl -n powerdns apply -f - << EOF
apiVersion: externaldns.k8s.io/v1alpha1
kind: DNSEndpoint
metadata:
  name: '{{ $.Setup.BaseDNSName }}-pair-sharing-io'
spec:
  endpoints:
  - dnsName: '{{ $.Setup.BaseDNSName }}'
    recordTTL: 60
    recordType: A
    targets:
    - ${LOAD_BALANCER_IP}
  - dnsName: '*.{{ $.Setup.BaseDNSName }}'
    recordTTL: 60
    recordType: A
    targets:
    - ${LOAD_BALANCER_IP}
  - dnsName: '{{ $.Setup.BaseDNSName }}'
    recordTTL: 60
    recordType: SOA
    targets:
    - 'ns1.{{ $.Setup.BaseDNSName }}. hostmaster.{{ $.Setup.BaseDNSName }}. 5 60 60 60 60'
EOF

cat << EOF | kubectl apply -f -
apiVersion: helm.fluxcd.io/v1
kind: HelmRelease
metadata:
  name: kubed
  namespace: kube-system
spec:
  releaseName: kubed
  chart:
    repository: https://charts.appscode.com/stable/
    name: kubed
    version: 0.12.0
  values:
    enableAnalytics: false
EOF
`,
						`
kubectl create ns "{{ $.Setup.UserLowercase }}"
kubectl label ns "{{ $.Setup.UserLowercase }}" cert-manager-tls=sync
mkdir -p /var/local/humacs-home-ii
chown 1000:1000 -R /var/local/humacs-home-ii
kubectl -n "{{ $.Setup.UserLowercase }}" apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: humacs-home-ii
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 500Gi
  storageClassName: local-path
EOF

cat << EOF | kubectl apply -f -
apiVersion: helm.fluxcd.io/v1
kind: HelmRelease
metadata:
  name: {{ $.Setup.UserLowercase }}
  namespace: {{ $.Setup.UserLowercase }}
spec:
  releaseName: {{ $.Setup.UserLowercase }}
  chart:
    git: https://github.com/humacs/humacs
    ref: eac3cae8f56c93116ae97136a4401980731d26bd
    path: chart/humacs
  values:
    image:
      repository: {{ $.Setup.HumacsRepository }}
      tag: {{ $.Setup.HumacsVersion }}
    options:
      hostDockerSocket: true
      hostTmp: true
      timezone: {{ $.Setup.Timezone }}
      gitName: {{ $.Setup.Fullname }}
      gitEmail: {{ $.Setup.Email }}
      profile: ""
      repos:
        {{ range $.Setup.Repos}}- {{ . }}
        {{ end }}
      preinitScript: |
        cat << EOF >> \$HOME/.gitconfig
        [credential "https://github.com"]
          helper = "!f() { test \\"\\\$1\\" = get && echo \\"password=\\\$GITHUB_TOKEN\\nusername=\\\$SHARINGIO_PAIR_USER\\";}; f"
        EOF
        git config --global commit.template \$HOME/.git-commit-template
        cat << EOF > \$HOME/.git-commit-template



        EOF
        for GUEST_NAME in \$SHARINGIO_PAIR_GUEST_NAMES; do
          echo "Co-Authored-By: \$GUEST_NAME <\$GUEST_NAME@users.noreply.github.com>" >> \$HOME/.git-commit-template
        done
        mkdir -p \$HOME/public_html
        echo "Add your site in '\$HOME/public_html'" > \$HOME/public_html/index.html
        git clone --depth=1 git://github.com/{{ $.Setup.User }}/.sharing.io || \
          git clone --depth=1 git://github.com/sharingio/.sharing.io
        (
          ./.sharing.io/init || true
        ) &
        for repo in \$(find ~ -type d -name ".git"); do
          repoName=\$(basename \$(dirname \$repo))
          if [ -x \$HOME/.sharing.io/\$repoName/init ]; then
            cd \$repo/..
            \$HOME/.sharing.io/\$repoName/init &
            continue
          fi
          if [ -x \$repo/../.sharing.io/init ]; then
            cd \$repo/..
            ./.sharing.io/init &
          fi
        done
    extraEnvVars:
      - name: SHARINGIO_PAIR_NAME
        value: {{ $.Name }}
      - name: SHARINGIO_PAIR_USER
        value: {{ $.Setup.User }}
      - name: SHARINGIO_PAIR_LOAD_BALANCER_IP
        value: ${LOAD_BALANCER_IP}
      - name: HUMACS_DEBUG
        value: "true"
      - name: REINIT_HOME_FOLDER
        value: "true"
      - name: SHARINGIO_PAIR_BASE_DNS_NAME
        value: {{ $.Setup.BaseDNSName }}
      - name: GITHUB_TOKEN
        value: {{ $.Setup.GitHubOAuthToken }}
      {{- if $.Setup.Env }}{{ range $index, $map := $.Setup.Env }}{{ range $key, $value := $map }}
      - name: {{ $key }}
        value: "{{ $value }}"       {{ end }}{{ end }}{{- end }}
    extraVolumes:
      - name: home-ii
        persistentVolumeClaim:
          claimName: humacs-home-ii
      - name: host
        hostPath:
          path: /
    extraVolumeMounts:
      - name: home-ii
        mountPath: /home/ii
      - name: host
        mountPath: /var/run/host
EOF

cat << EOF | kubectl apply -f -
apiVersion: helm.fluxcd.io/v1
kind: HelmRelease
metadata:
  name: public-html
  namespace: {{ $.Setup.UserLowercase }}
spec:
  releaseName: public-html
  chart:
    git: https://gitlab.com/safesurfer/go-http-server
    ref: 1.2.0
    path: deployments/go-http-server
  values:
    serveFolder: /home/ii/public_html
    vuejsHistoryMode: false
    image:
      tag: 1.2.0
    ingress:
      enabled: true
      realIPHeader: "X-Real-Ip"
      hosts:
        - host: www.{{ $.Setup.BaseDNSName }}
          paths:
            - /
      tls:
        - secretName: letsencrypt-prod
          hosts:
            - www.{{ $.Setup.BaseDNSName }}
    extraVolumeMounts:
      - name: humacs-home-ii
        mountPath: /home/ii
      - name: host
        mountPath: /var/run/host
    extraVolumes:
      - name: humacs-home-ii
        persistentVolumeClaim:
          claimName: humacs-home-ii
      - name: host
        hostPath:
          path: /
EOF

export BASE_DNS_NAME={{ $.Setup.BaseDNSName }}
`,
						`
  kubectl -n powerdns wait pod --for=condition=Ready --selector=app.kubernetes.io/name=powerdns --timeout=200s
  until [ "$(dig A ${BASE_DNS_NAME} +short)" = "${LOAD_BALANCER_IP}" ]; do
    echo "BaseDNSName does not resolve to Instance IP yet"
    sleep 1
  done
  kubectl -n powerdns exec deployment/powerdns -- pdnsutil generate-tsig-key pair hmac-md5
  kubectl -n powerdns exec deployment/powerdns -- pdnsutil activate-tsig-key ${BASE_DNS_NAME} pair master
  kubectl -n powerdns exec deployment/powerdns -- pdnsutil set-meta ${BASE_DNS_NAME} TSIG-ALLOW-DNSUPDATE pair
  kubectl -n powerdns exec deployment/powerdns -- pdnsutil set-meta ${BASE_DNS_NAME} NOTIFY-DNSUPDATE 1
  kubectl -n powerdns exec deployment/powerdns -- pdnsutil set-meta ${BASE_DNS_NAME} SOA-EDIT-DNSUPDATE EPOCH
  export POWERDNS_TSIG_SECRET="$(kubectl -n powerdns exec deployment/powerdns -- pdnsutil list-tsig-keys | grep pair | awk '{print $3}')"
  nsupdate <<EOF
server ${LOAD_BALANCER_IP} 53
zone ${BASE_DNS_NAME}
update add ${BASE_DNS_NAME} 60 NS ns1.{{ $.Setup.BaseDNSName }}
key pair ${POWERDNS_TSIG_SECRET}
send
EOF

  kubectl -n cert-manager create secret generic tsig-powerdns --from-literal=powerdns="$POWERDNS_TSIG_SECRET"
  kubectl -n powerdns create secret generic tsig-powerdns --from-literal=powerdns="$POWERDNS_TSIG_SECRET"
  kubectl -n powerdns apply -f - << EOF
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: {{ $.Setup.Email }}
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - dns01:
        rfc2136:
          tsigKeyName: pair
          tsigAlgorithm: HMACMD5
          tsigSecretSecretRef:
            name: tsig-powerdns
            key: powerdns
          nameserver: ${LOAD_BALANCER_IP}
      selector:
        dnsNames:
          - "*.${BASE_DNS_NAME}"
          - "${BASE_DNS_NAME}"
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: letsencrypt-prod
spec:
  secretName: letsencrypt-prod
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
  commonName: "*.${BASE_DNS_NAME}"
  dnsNames:
    - "*.${BASE_DNS_NAME}"
    - "${BASE_DNS_NAME}"
EOF
(
   while true; do
     conditions=$(kubectl -n powerdns get cert letsencrypt-prod -o=jsonpath='{.status.conditions[0]}')
     if [ "$(echo $conditions | jq -r .type)" = "Ready" ] && [ "$(echo $conditions | jq -r .status)" = "True" ]; then
       break
     fi
     echo "Waiting for valid TLS cert"
     sleep 1
   done
   kubectl -n powerdns annotate secret letsencrypt-prod kubed.appscode.com/sync=cert-manager-tls --overwrite
) &
`,
						"kubectl -n default create configmap sharingio-pair-init-complete",
					},
				},
			},
		},
		Cluster: clusterAPIv1alpha3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "",
				Labels: map[string]string{"io.sharing.pair": "instance"},
			},
			Spec: clusterAPIv1alpha3.ClusterSpec{
				ClusterNetwork: &clusterAPIv1alpha3.ClusterNetwork{
					Pods: &clusterAPIv1alpha3.NetworkRanges{
						CIDRBlocks: []string{
							"10.244.0.0/16",
						},
					},
					Services: &clusterAPIv1alpha3.NetworkRanges{
						CIDRBlocks: []string{
							"10.96.0.0/12",
						},
					},
				},
				InfrastructureRef: &corev1.ObjectReference{
					APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha3",
					Kind:       "PacketCluster",
				},
				ControlPlaneRef: &corev1.ObjectReference{
					APIVersion: "controlplane.cluster.x-k8s.io/v1alpha3",
					Kind:       "KubeadmControlPlane",
				},
			},
		},
		MachineDeploymentWorker: clusterAPIv1alpha3.MachineDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "",
				Labels: map[string]string{
					"pool":            "worker-a",
					"io.sharing.pair": "instance",
				},
			},
			Spec: clusterAPIv1alpha3.MachineDeploymentSpec{
				Replicas:    Int32ToInt32Pointer(0),
				ClusterName: "",
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pool": "worker-a",
					},
				},
				Template: clusterAPIv1alpha3.MachineTemplateSpec{
					ObjectMeta: clusterAPIv1alpha3.ObjectMeta{
						Name: "",
						Labels: map[string]string{
							"io.sharing.pair": "instance",
							"pool":            "worker-a",
						},
					},
					Spec: clusterAPIv1alpha3.MachineSpec{
						Version:     &instance.Setup.KubernetesVersion,
						ClusterName: "",
						Bootstrap: clusterAPIv1alpha3.Bootstrap{
							ConfigRef: &corev1.ObjectReference{
								APIVersion: "bootstrap.cluster.x-k8s.io/v1alpha3",
								Kind:       "KubeadmConfigTemplate",
							},
						},
						InfrastructureRef: corev1.ObjectReference{
							APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha3",
							Kind:       "PacketMachineTemplate",
						},
					},
				},
			},
		},
		KubeadmConfigTemplateWorker: cabpkv1.KubeadmConfigTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "",
				Labels: map[string]string{"io.sharing.pair": "instance"},
			},
			Spec: cabpkv1.KubeadmConfigTemplateSpec{
				Template: cabpkv1.KubeadmConfigTemplateResource{
					Spec: cabpkv1.KubeadmConfigSpec{
						PreKubeadmCommands: []string{
							"sed -ri '/\\sswap\\s/s/^#?/#/' /etc/fstab",
							"swapoff -a",
							"mount -a",
							"apt-get -y update",
							"DEBIAN_FRONTEND=noninteractive apt-get install -y apt-transport-https curl",
							"curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key add -",
							"echo \"deb https://apt.kubernetes.io/ kubernetes-xenial main\" > /etc/apt/sources.list.d/kubernetes.list",
							"curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -",
							"apt-key fingerprint 0EBFCD88",
							"add-apt-repository \"deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable\"",
							"apt-get update -y",
							"apt-get install -y ca-certificates socat jq ebtables apt-transport-https cloud-utils prips docker-ce docker-ce-cli containerd.io kubelet kubeadm kubectl ssh-import-id dnsutils kitty-terminfo git",
							`cat <<EOF > /etc/docker/daemon.json
{
  "containerd-namespace": "k8s.io",
  "containerd-plugins-namespace": "k8s.io",
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "500m",
    "max-file": "3"
  }
}
EOF`,
							"systemctl daemon-reload",
							"systemctl enable docker",
							"systemctl start docker",
							"chgrp users /var/run/docker.sock",
							`cat <<EOF | tee /etc/modules-load.d/containerd.conf
overlay
br_netfilter
EOF
modprobe overlay
modprobe br_netfilter
cat <<EOF | tee /etc/sysctl.d/99-kubernetes-cri.conf
net.bridge.bridge-nf-call-iptables  = 1
net.ipv4.ip_forward                 = 1
net.bridge.bridge-nf-call-ip6tables = 1
EOF
sysctl --system
`,
							"mkdir -p /etc/containerd",
							"rm /etc/containerd/config.toml",
							"systemctl restart containerd",
							"export CONTAINER_RUNTIME_ENDPOINT=/var/run/containerd/containerd.sock",
						},
						JoinConfiguration: &kubeadmv1beta1.JoinConfiguration{
							NodeRegistration: kubeadmv1beta1.NodeRegistrationOptions{
								KubeletExtraArgs: map[string]string{
									"cloud-provider": "external",
								},
							},
						},
					},
				},
			},
		},
		PacketMachineTemplate: clusterAPIPacketv1alpha3.PacketMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "",
				Labels: map[string]string{"io.sharing.pair": "instance"},
			},
			Spec: clusterAPIPacketv1alpha3.PacketMachineTemplateSpec{
				Template: clusterAPIPacketv1alpha3.PacketMachineTemplateResource{
					Spec: clusterAPIPacketv1alpha3.PacketMachineSpec{
						OS:           defaultMachineOS,
						BillingCycle: "hourly",
						// 1 = machine type
						MachineType: "",
					},
				},
			},
		},
		PacketCluster: clusterAPIPacketv1alpha3.PacketCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "",
				Labels: map[string]string{"io.sharing.pair": "instance"},
			},
			Spec: clusterAPIPacketv1alpha3.PacketClusterSpec{
				ControlPlaneEndpoint: clusterAPIv1alpha3.APIEndpoint{
					Host: "sharing.io",
					Port: 6443,
				},
			},
		},
		PacketMachineTemplateWorker: clusterAPIPacketv1alpha3.PacketMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "",
				Labels: map[string]string{"io.sharing.pair": "instance"},
			},
			Spec: clusterAPIPacketv1alpha3.PacketMachineTemplateSpec{
				Template: clusterAPIPacketv1alpha3.PacketMachineTemplateResource{
					Spec: clusterAPIPacketv1alpha3.PacketMachineSpec{
						OS:           defaultMachineOS,
						BillingCycle: "hourly",
						// 1 = machine type
						MachineType: "",
					},
				},
			},
		},
	}
	instance.Setup.BaseDNSName = instance.Name + "." + common.GetBaseHost()
	instance.Setup.GuestsNamesFlat = strings.Join(instance.Setup.Guests, " ")
	newInstance = defaultKubernetesClusterConfig
	newInstance.KubeadmControlPlane.ObjectMeta.Name = instance.Name + "-control-plane"
	newInstance.KubeadmControlPlane.ObjectMeta.Namespace = namespace
	newInstance.KubeadmControlPlane.ObjectMeta.Annotations = map[string]string{}
	newInstance.KubeadmControlPlane.ObjectMeta.Annotations["io.sharing.pair-spec-name"] = instance.Name
	newInstance.KubeadmControlPlane.ObjectMeta.Annotations["io.sharing.pair-spec-setup-user"] = instance.Setup.User
	newInstance.KubeadmControlPlane.Spec.InfrastructureTemplate.Name = instance.Name + "-control-plane"

	tmpl, err := template.New(fmt.Sprintf("ssh-keys-%s-%v", instance.Name, time.Now().Unix())).Parse(defaultKubernetesClusterConfig.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands[6])
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Error templating ssh-keys commands: %#v", err), newInstance
	}
	templatedBuffer := new(bytes.Buffer)
	err = tmpl.Execute(templatedBuffer, instance)
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Error templating ssh-keys commands: %#v", err), newInstance
	}
	newInstance.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands[6] = templatedBuffer.String()

	tmpl, err = template.New(fmt.Sprintf("packet-cloud-config-secret-%s-%v", instance.Name, time.Now().Unix())).Parse(defaultKubernetesClusterConfig.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands[9])
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Error templating packet-cloud-config-secret command: %#v", err), newInstance
	}
	templatedBuffer = new(bytes.Buffer)
	err = tmpl.Execute(templatedBuffer, map[string]interface{}{
		"PacketProjectID": common.GetPacketProjectID(),
		"InstanceName":    instance.Name,
		// NOTE I could find a way to ignore this field during templating, here's a neat little hack to ignore it ;)
		"apiKey": "{{ .apiKey }}",
	})
	if err != nil {
		log.Printf("%#v\n", err.Error())
		return fmt.Errorf("Error templating packet-cloud-config-secret command: %#v", err), newInstance
	}
	newInstance.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands[9] = templatedBuffer.String()

	fmt.Printf("\n\n\nTemplate name: external-dns-%v\nInstance: %#v\n\n\n", instance.Name, time.Now().Unix(), instance)
	tmpl, err = template.New(fmt.Sprintf("external-dns-%s-%v", instance.Name, time.Now().Unix())).Parse(defaultKubernetesClusterConfig.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands[22])
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Error templating External DNS install command: %#v", err), newInstance
	}
	templatedBuffer = new(bytes.Buffer)
	err = tmpl.Execute(templatedBuffer, instance)
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Error templating External DNS install command: %#v", err), newInstance
	}
	newInstance.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands[22] = templatedBuffer.String()

	fmt.Printf("\n\n\nTemplate name: powerdns%v\nInstance: %#v\n\n\n", instance.Name, time.Now().Unix(), instance)
	tmpl, err = template.New(fmt.Sprintf("powerdns-%s-%v", instance.Name, time.Now().Unix())).Funcs(TemplateFuncMap()).Parse(defaultKubernetesClusterConfig.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands[23])
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Error templating PowerDNS install command: %#v", err), newInstance
	}
	templatedBuffer = new(bytes.Buffer)
	err = tmpl.Execute(templatedBuffer, instance)
	if err != nil {
		log.Printf("%#v; %v\n", err, err.Error())
		return fmt.Errorf("Error templating PowerDNS install command: %#v", err), newInstance
	}
	newInstance.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands[23] = templatedBuffer.String()

	fmt.Printf("\n\n\nTemplate name: humacs-helm-install-%s-%v\nInstance: %#v\n\n\n", instance.Name, time.Now().Unix(), instance)
	tmpl, err = template.New(fmt.Sprintf("humacs-helm-install-%s-%v", instance.Name, time.Now().Unix())).Funcs(TemplateFuncMap()).Parse(defaultKubernetesClusterConfig.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands[24])
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Error templating Humacs Helm install command: %#v", err), newInstance
	}
	templatedBuffer = new(bytes.Buffer)
	err = tmpl.Execute(templatedBuffer, instance)
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Error templating Humacs Helm install command: %#v", err), newInstance
	}
	newInstance.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands[24] = templatedBuffer.String()

	fmt.Printf("\n\n\nTemplate name: certs-%s-%v\nInstance: %#v\n\n\n", instance.Name, time.Now().Unix(), instance)
	tmpl, err = template.New(fmt.Sprintf("certs-%v-%v", instance.Name, time.Now().Unix())).Parse(defaultKubernetesClusterConfig.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands[25])
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Error templating certs command: %#v", err), newInstance
	}
	templatedBuffer = new(bytes.Buffer)
	err = tmpl.Execute(templatedBuffer, instance)
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Error templating certs command: %#v", err), newInstance
	}
	newInstance.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands[25] = templatedBuffer.String()

	templatedBuffer = nil
	tmpl = nil

	newInstance.PacketMachineTemplate.ObjectMeta.Name = instance.Name + "-control-plane"
	newInstance.PacketMachineTemplate.ObjectMeta.Namespace = namespace
	newInstance.PacketMachineTemplate.ObjectMeta.Annotations = map[string]string{}
	newInstance.PacketMachineTemplate.ObjectMeta.Annotations["io.sharing.pair-spec-name"] = instance.Name
	newInstance.PacketMachineTemplate.ObjectMeta.Annotations["io.sharing.pair-spec-setup-user"] = instance.Setup.User
	// TODO default value configuration scope - deployment based configuration
	newInstance.PacketMachineTemplate.Spec.Template.Spec.MachineType = instance.NodeSize

	newInstance.MachineDeploymentWorker.ObjectMeta.Name = instance.Name + "-worker-a"
	newInstance.MachineDeploymentWorker.ObjectMeta.Namespace = namespace
	newInstance.MachineDeploymentWorker.ObjectMeta.Annotations = map[string]string{}
	newInstance.MachineDeploymentWorker.ObjectMeta.Annotations["cluster.x-k8s.io/cluster-name"] = instance.Name
	newInstance.MachineDeploymentWorker.ObjectMeta.Annotations["io.sharing.pair-spec-name"] = instance.Name
	newInstance.MachineDeploymentWorker.ObjectMeta.Annotations["io.sharing.pair-spec-setup-user"] = instance.Setup.User
	newInstance.MachineDeploymentWorker.Spec.ClusterName = instance.Name
	newInstance.MachineDeploymentWorker.Spec.Template.Spec.Bootstrap.ConfigRef.Name = instance.Name + "-worker-a"
	newInstance.MachineDeploymentWorker.Spec.Selector.MatchLabels["cluster.x-k8s.io/cluster-name"] = instance.Name
	newInstance.MachineDeploymentWorker.Spec.Template.ObjectMeta.Annotations = map[string]string{}
	newInstance.MachineDeploymentWorker.Spec.Template.ObjectMeta.Annotations["cluster.x-k8s.io/cluster-name"] = instance.Name
	newInstance.MachineDeploymentWorker.Spec.Template.ObjectMeta.Annotations["io.sharing.pair-spec-name"] = instance.Name
	newInstance.MachineDeploymentWorker.Spec.Template.ObjectMeta.Annotations["io.sharing.pair-spec-setup-user"] = instance.Setup.User
	newInstance.MachineDeploymentWorker.Spec.Template.Spec.InfrastructureRef.Name = instance.Name + "-worker-a"
	newInstance.MachineDeploymentWorker.Spec.Template.Spec.ClusterName = instance.Name

	newInstance.PacketCluster.ObjectMeta.Name = instance.Name
	newInstance.PacketCluster.ObjectMeta.Namespace = namespace
	newInstance.PacketCluster.ObjectMeta.Annotations = map[string]string{}
	newInstance.PacketCluster.ObjectMeta.Annotations["io.sharing.pair-spec-name"] = instance.Name
	newInstance.PacketCluster.ObjectMeta.Annotations["io.sharing.pair-spec-setup-user"] = instance.Setup.User
	// TODO default value configuration scope - deployment based configuration
	newInstance.PacketCluster.Spec.ProjectID = common.GetPacketProjectID()
	newInstance.PacketCluster.Spec.Facility = instance.Facility

	newInstance.Cluster.ObjectMeta.Name = instance.Name
	newInstance.Cluster.ObjectMeta.Namespace = namespace
	newInstance.Cluster.ObjectMeta.Annotations = map[string]string{}
	newInstance.Cluster.ObjectMeta.Annotations["io.sharing.pair-spec-name"] = instance.Name
	newInstance.Cluster.ObjectMeta.Annotations["io.sharing.pair-spec-nodeSize"] = instance.NodeSize
	newInstance.Cluster.ObjectMeta.Annotations["io.sharing.pair-spec-facility"] = instance.Facility
	newInstance.Cluster.ObjectMeta.Annotations["io.sharing.pair-spec-setup-user"] = instance.Setup.User
	newInstance.Cluster.ObjectMeta.Annotations["io.sharing.pair-spec-setup-guests"] = strings.Join(instance.Setup.Guests, " ")
	newInstance.Cluster.ObjectMeta.Annotations["io.sharing.pair-spec-setup-repos"] = strings.Join(instance.Setup.Repos, " ")
	newInstance.Cluster.ObjectMeta.Annotations["io.sharing.pair-spec-setup-timezone"] = instance.Setup.Timezone
	newInstance.Cluster.ObjectMeta.Annotations["io.sharing.pair-spec-setup-fullname"] = instance.Setup.Fullname
	newInstance.Cluster.ObjectMeta.Annotations["io.sharing.pair-spec-setup-email"] = instance.Setup.Email
	newInstance.Cluster.ObjectMeta.Annotations["io.sharing.pair-spec-setup-baseDNSName"] = instance.Setup.BaseDNSName
	envJSON, err := json.Marshal(instance.Setup.Env)
	if err != nil {
		log.Printf("%#v\n", err)
		return err, newInstance
	}
	newInstance.Cluster.ObjectMeta.Annotations["io.sharing.pair-spec-setup-env"] = string(envJSON)
	newInstance.Cluster.Spec.InfrastructureRef.Name = instance.Name
	newInstance.Cluster.Spec.ControlPlaneRef.Name = instance.Name + "-control-plane"

	newInstance.KubeadmConfigTemplateWorker.ObjectMeta.Name = instance.Name + "-worker-a"
	newInstance.KubeadmConfigTemplateWorker.ObjectMeta.Namespace = namespace
	newInstance.KubeadmConfigTemplateWorker.ObjectMeta.Annotations = map[string]string{}
	newInstance.KubeadmConfigTemplateWorker.ObjectMeta.Annotations["io.sharing.pair-spec-name"] = instance.Name
	newInstance.KubeadmConfigTemplateWorker.ObjectMeta.Annotations["io.sharing.pair-spec-setup-user"] = instance.Setup.User

	newInstance.PacketMachineTemplateWorker.ObjectMeta.Name = instance.Name + "-worker-a"
	newInstance.PacketMachineTemplateWorker.ObjectMeta.Namespace = namespace
	newInstance.PacketMachineTemplateWorker.ObjectMeta.Annotations = map[string]string{}
	newInstance.PacketMachineTemplateWorker.ObjectMeta.Annotations["io.sharing.pair-spec-name"] = instance.Name
	newInstance.PacketMachineTemplateWorker.ObjectMeta.Annotations["io.sharing.pair-spec-setup-user"] = instance.Setup.User
	// TODO default value configuration scope - deployment based configuration
	newInstance.PacketMachineTemplateWorker.Spec.Template.Spec.MachineType = instance.NodeSize

	return err, newInstance
}

// KubernetesGetKubeconfigBytes ...
// given an instance name and clientset, return the instance's kubeconfig as bytes
func KubernetesGetKubeconfigBytes(name string, clientset *kubernetes.Clientset) (err error, kubeconfigBytes []byte) {
	targetNamespace := common.GetTargetNamespace()
	secret, err := clientset.CoreV1().Secrets(targetNamespace).Get(context.TODO(), fmt.Sprintf("%s-kubeconfig", name), metav1.GetOptions{})
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to get Kubernetes cluster Kubeconfig; err: %#v", err), []byte{}
	}
	kubeconfigBytes = secret.Data["value"]
	return err, kubeconfigBytes
}

// KubernetesGetKubeconfigYAML ...
// given an instance name and clientset, return the instance's kubeconfig as YAML
func KubernetesGetKubeconfigYAML(name string, clientset *kubernetes.Clientset) (err error, kubeconfig string) {
	err, kubeconfigBytes := KubernetesGetKubeconfigBytes(name, clientset)
	if err != nil {
		return err, kubeconfig
	}
	return err, string(kubeconfigBytes)
}

// KubernetesDynamicGetKubeconfigBytes ...
// given an instance name and dynamic client, return the instance's kubeconfig as bytes
func KubernetesDynamicGetKubeconfigBytes(name string, kubernetesClientset dynamic.Interface) (err error, kubeconfig []byte) {
	targetNamespace := common.GetTargetNamespace()
	groupVersionResource := schema.GroupVersionResource{Version: "v1", Group: "", Resource: "secrets"}
	secret, err := kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Get(context.TODO(), fmt.Sprintf("%s-kubeconfig", name), metav1.GetOptions{})
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to get Kubernetes cluster Kubeconfig; err: %#v", err), []byte{}
	}
	var itemRestructured corev1.Secret
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(secret.Object, &itemRestructured)
	if err != nil {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to restructure %T; err: %v", itemRestructured, err), []byte{}
	}
	kubeconfigBytes := itemRestructured.Data["value"]
	return err, kubeconfigBytes
}

// KubernetesGetKubeconfig ...
// given an instance name and clientset, return a kubeconfig for clientset
func KubernetesGetKubeconfig(name string, clientset *kubernetes.Clientset) (err error, kubeconfig *clientcmdapi.Config) {
	err, valueBytes := KubernetesGetKubeconfigBytes(name, clientset)
	kubeconfig, err = clientcmd.Load(valueBytes)
	return err, kubeconfig
}

// KubernetesExec ...
// exec a command in an Instance Kubernetes Pod
func KubernetesExec(clientset *kubernetes.Clientset, restConfig *rest.Config, options ExecOptions) (err error, stdout string, stderr string) {
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(fmt.Sprintf("%s-humacs-0", options.PodName)).
		Namespace(options.Namespace).
		SubResource("exec").
		Param("container", options.ContainerName)
	req.VersionedParams(&corev1.PodExecOptions{
		Container: options.ContainerName,
		Command:   options.Command,
		Stdin:     options.Stdin != nil,
		Stdout:    options.CaptureStdout,
		Stderr:    options.CaptureStderr,
		TTY:       options.TTY,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
	if err != nil {
		return err, stdout, stderr
	}
	var stdoutBuffer, stderrBuffer bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  options.Stdin,
		Stdout: &stdoutBuffer,
		Stderr: &stderrBuffer,
		Tty:    options.TTY,
	})
	if err != nil {
		return err, stdoutBuffer.String(), stderrBuffer.String()
	}

	if options.PreserveWhitespace {
		return err, stdoutBuffer.String(), stderrBuffer.String()
	}
	return err, strings.TrimSpace(stdoutBuffer.String()), strings.TrimSpace(stderrBuffer.String())

	// https://github.com/kubernetes/kubectl/blob/e65caf964573fbf671c4648032da4b7df7c7eaf0/pkg/cmd/exec/exec.go#L357
}

// KubernetesGetTmateSSHSession ...
// given a clienset, instancename, and username, get the tmate SSH session for the Humacs Pod
func KubernetesGetTmateSSHSession(clientset *kubernetes.Clientset, instanceName string, userName string) (err error, output string) {
	err, instanceKubeconfig := KubernetesGetKubeconfigBytes(instanceName, clientset)
	if err != nil {
		return err, output
	}
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(instanceKubeconfig)
	if err != nil {
		return err, output
	}
	err, instanceClientset := KubernetesClientsetFromKubeconfigBytes(instanceKubeconfig)
	if err != nil {
		return err, output
	}

	execOptions := ExecOptions{
		Command: []string{
			"tmate",
			"-S",
			"/tmp/ii.default.target.iisocket",
			"display",
			"-p",
			"#{tmate_ssh}",
		},
		Namespace:          userName,
		PodName:            userName,
		ContainerName:      "humacs",
		CaptureStderr:      true,
		CaptureStdout:      true,
		PreserveWhitespace: false,
		TTY:                false,
	}
	err, stdout, stderr := KubernetesExec(instanceClientset, restConfig, execOptions)
	if stderr != "" {
		return fmt.Errorf(stderr), stdout
	}
	return err, stdout
}

// KubernetesGetTmateWebSession ...
// given a clienset, instancename, and username, get the tmate web session for the Humacs Pod
func KubernetesGetTmateWebSession(clientset *kubernetes.Clientset, instanceName string, userName string) (err error, output string) {
	err, instanceKubeconfig := KubernetesGetKubeconfigBytes(instanceName, clientset)
	if err != nil {
		return err, output
	}
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(instanceKubeconfig)
	if err != nil {
		return err, output
	}
	err, instanceClientset := KubernetesClientsetFromKubeconfigBytes(instanceKubeconfig)
	if err != nil {
		return err, output
	}

	execOptions := ExecOptions{
		Command: []string{
			"tmate",
			"-S",
			"/tmp/ii.default.target.iisocket",
			"display",
			"-p",
			"#{tmate_web}",
		},
		Namespace:          userName,
		PodName:            userName,
		ContainerName:      "humacs",
		CaptureStderr:      true,
		CaptureStdout:      true,
		PreserveWhitespace: false,
		TTY:                false,
	}
	err, stdout, stderr := KubernetesExec(instanceClientset, restConfig, execOptions)
	if stderr != "" {
		return fmt.Errorf(stderr), stdout
	}
	return err, stdout
}

// KubernetesGetInstanceIngresses ...
// given a clienset and instance name, return the Ingresses available on the instance
func KubernetesGetInstanceIngresses(clientset *kubernetes.Clientset, instanceName string) (err error, ingresses *networkingv1.IngressList) {
	err, instanceKubeconfig := KubernetesGetKubeconfigBytes(instanceName, clientset)
	if err != nil {
		return err, &networkingv1.IngressList{}
	}
	err, instanceClientset := KubernetesClientsetFromKubeconfigBytes(instanceKubeconfig)
	if err != nil {
		return err, &networkingv1.IngressList{}
	}

	ingresses, err = instanceClientset.NetworkingV1().Ingresses("").List(context.TODO(), metav1.ListOptions{})
	return err, ingresses
}

// KubernetesClientsetFromKubeconfigBytes ...
// given an kubeconfig as a slice of bytes return a clientset
func KubernetesClientsetFromKubeconfigBytes(kubeconfigBytes []byte) (err error, clientset *kubernetes.Clientset) {
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigBytes)
	if err != nil {
		return err, &kubernetes.Clientset{}
	}
	clientset, err = kubernetes.NewForConfig(restConfig)
	return err, clientset
}

// KubernetesWaitForInstanceKubeconfig ...
// given a local clientset and instance name, wait for the instance kubeconfig to populate locally
func KubernetesWaitForInstanceKubeconfig(clientset *kubernetes.Clientset, instanceName string) {
	targetNamespace := common.GetTargetNamespace()
	kubeconfigName := fmt.Sprintf("%s-kubeconfig", instanceName)
pollInstanceNamespace:
	for true {
		deadline := time.Now().Add(time.Second * 3)
		ctx, _ := context.WithDeadline(context.TODO(), deadline)
		ns, err := clientset.CoreV1().Secrets(targetNamespace).Get(ctx, kubeconfigName, metav1.GetOptions{})
		if err == nil && ns.ObjectMeta.Name == kubeconfigName {
			log.Printf("Found Secret '%v' in Namespace '%v'\n", kubeconfigName, targetNamespace)
			break pollInstanceNamespace
		}
		log.Printf("Failed to find Secret '%v' in Namespace '%v', %v\n", kubeconfigName, targetNamespace, err)
		time.Sleep(time.Second * 5)
	}
}

// KubernetesAddMachineIPToDNS ...
// given a dynamicClient, instance name, and subdomain,
// wait for machine IP and upsert the DNS endpoint with the external provider
func KubernetesAddMachineIPToDNS(dynamicClient dynamic.Interface, name string, subdomain string) (err error) {
	targetNamespace := common.GetTargetNamespace()
	var ipAddress string
	groupVersion := clusterAPIv1alpha3.GroupVersion
	groupVersionResource := schema.GroupVersionResource{Version: groupVersion.Version, Group: "cluster.x-k8s.io", Resource: "machines"}
	log.Printf("%#v\n", groupVersionResource)
	log.Println("watching instance machine")
	watcher, err := dynamicClient.Resource(groupVersionResource).Namespace(targetNamespace).Watch(context.TODO(), metav1.ListOptions{LabelSelector: "cluster.x-k8s.io/cluster-name=" + name})
	if err != nil {
		log.Printf("%#v\n", err)
		return err
	}
	defer watcher.Stop()
	watchChan := watcher.ResultChan()
machineWatchChannel:
	for event := range watchChan {
		log.Println("machine event received")
		eventObjectBytes, _ := json.Marshal(event.Object)
		var machine clusterAPIv1alpha3.Machine
		json.Unmarshal(eventObjectBytes, &machine)
		fmt.Printf("%#v\n", machine)
		if len(machine.Status.Addresses) < 1 {
			log.Println("error: machine has no IP addresses")
			continue
		}
		if machine.Status.Addresses[1].Address == "" {
			log.Println("error: machine address is empty")
			continue
		}
		if govalidator.IsIPv4(machine.Status.Addresses[1].Address) == false {
			log.Printf("error '%v' is not a valid IPv4 address", machine.Status.Addresses[1].Address)
			continue
		}

		// NOTE first IP doesn't work, as it's used for the cluster's API; instead we will use the second, which works
		ipAddress = machine.Status.Addresses[1].Address
		fmt.Println("machine IP available:", ipAddress)
		break machineWatchChannel
	}
	entry := dns.Entry{
		Subdomain: subdomain,
		Values: []string{
			ipAddress,
		},
	}
	err = dns.UpsertDNSEndpoint(dynamicClient, entry, name)
	if err != nil {
		log.Printf("%#v\n", err)
	}

	return err
}

// KubernetesGetInstanceWildcardTLSCert ...
// given an instance clientset and instance name, return a TLS wildcard cert
func KubernetesGetInstanceWildcardTLSCert(clientset *kubernetes.Clientset, instanceName string) (err error, secret *corev1.Secret) {
	targetNamespace := "powerdns"
	templatedSecretName := "letsencrypt-prod"
	err, instanceKubeconfig := KubernetesGetKubeconfigBytes(instanceName, clientset)
	if err != nil {
		return err, &corev1.Secret{}
	}
	err, instanceClientset := KubernetesClientsetFromKubeconfigBytes(instanceKubeconfig)
	if err != nil {
		return err, &corev1.Secret{}
	}

	secret, err = instanceClientset.CoreV1().Secrets(targetNamespace).Get(context.TODO(), templatedSecretName, metav1.GetOptions{})
	return err, secret
}

// KubernetesGetLocalInstanceWildcardTLSCert ...
// given a local clientset and instance name, return the local TLS wildcard cert
func KubernetesGetLocalInstanceWildcardTLSCert(clientset *kubernetes.Clientset, username string) (err error, secret *corev1.Secret) {
	targetNamespace := common.GetTargetNamespace()
	templatedSecretName := fmt.Sprintf("%v-tls", username)

	secret, err = clientset.CoreV1().Secrets(targetNamespace).Get(context.TODO(), templatedSecretName, metav1.GetOptions{})
	if secret.ObjectMeta.Name == templatedSecretName {
		log.Printf("Found secret '%v' in namespace '%v'\n", templatedSecretName, targetNamespace)
	}
	return err, secret
}

// KubernetesUpsertLocalInstanceWildcardTLSCert ...
// given a local clientset, username, and secret,
// locally upsert the secret
func KubernetesUpsertLocalInstanceWildcardTLSCert(clientset *kubernetes.Clientset, username string, secret *corev1.Secret) (err error) {
	targetNamespace := common.GetTargetNamespace()
	templatedSecretName := fmt.Sprintf("%v-tls", username)
	templatedSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: templatedSecretName,
			Labels: map[string]string{
				"io.sharing.pair": "instance",
			},
			Annotations: secret.ObjectMeta.Annotations,
		},
		Type: corev1.SecretTypeTLS,
		Data: secret.Data,
	}
	_, err = clientset.CoreV1().Secrets(targetNamespace).Create(context.TODO(), &templatedSecret, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		err = nil
		existingSecret, err := clientset.CoreV1().Secrets(targetNamespace).Get(context.TODO(), templatedSecretName, metav1.GetOptions{})
		if err != nil {
			log.Printf("%#v\n", err)
			return fmt.Errorf("Failed to get Secret '%v' in namespace '%v', %#v", templatedSecretName, targetNamespace, err)
		}
		templatedSecret.SetResourceVersion(existingSecret.GetResourceVersion())
		_, err = clientset.CoreV1().Secrets(targetNamespace).Update(context.TODO(), &templatedSecret, metav1.UpdateOptions{})
		if err != nil {
			log.Printf("%#v\n", err)
			return fmt.Errorf("Failed to update Secret '%v' in namespace '%v', %#v", templatedSecretName, targetNamespace, err)
		}
	} else {
		log.Printf("Created Secret '%v' in namespace '%v'", templatedSecretName, targetNamespace)
	}
	return err
}

// KubernetesUpsertInstanceWildcardTLSCert ...
// given an instance clientset, username, and secert,
// upsert the secret to the instance
func KubernetesUpsertInstanceWildcardTLSCert(clientset *kubernetes.Clientset, username string, secret *corev1.Secret) (err error) {
	targetNamespace := "powerdns"
	templatedSecretName := "letsencrypt-prod"
	templatedSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: templatedSecretName,
			Labels: map[string]string{
				"io.sharing.pair": "instance",
			},
			Annotations: secret.ObjectMeta.Annotations,
		},
		Type: corev1.SecretTypeTLS,
		Data: secret.Data,
	}
	_, err = clientset.CoreV1().Secrets(targetNamespace).Create(context.TODO(), &templatedSecret, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		log.Printf("Secret '%v' already exists in namespace '%v'", templatedSecretName, targetNamespace)
		err = nil
		existingSecret, err := clientset.CoreV1().Secrets(targetNamespace).Get(context.TODO(), templatedSecretName, metav1.GetOptions{})
		if err != nil {
			log.Printf("%#v\n", err)
			return fmt.Errorf("Failed to get Secret '%v' in namespace '%v', %#v", templatedSecretName, targetNamespace, err)
		}
		templatedSecret.SetResourceVersion(existingSecret.GetResourceVersion())
		log.Printf("Updating Secret '%v' in namespace '%v'", templatedSecretName, targetNamespace)
		_, err = clientset.CoreV1().Secrets(targetNamespace).Update(context.TODO(), &templatedSecret, metav1.UpdateOptions{})
		if err != nil {
			log.Printf("%#v\n", err)
			return fmt.Errorf("Failed to update Secret '%v' in namespace '%v', %#v", templatedSecretName, targetNamespace, err)
		}
	} else {
		log.Printf("Created Secret '%v' in namespace '%v'", templatedSecretName, targetNamespace)
	}
	return err
}

// KubernetesAddCertToMachine ...
// given a clientset, dynamic client, and instance name,
// manage the lifecycle of a cert on an instance
func KubernetesAddCertToMachine(clientset *kubernetes.Clientset, dynamicClient dynamic.Interface, instanceName string) (err error) {
	// if cert secret for user name exists locally
	namespace := "powerdns"
	log.Printf("Managing cert for Instance '%v'\n", instanceName)
	errLocalInstance, localSecret := KubernetesGetLocalInstanceWildcardTLSCert(clientset, instanceName)

	KubernetesWaitForInstanceKubeconfig(clientset, instanceName)

	err, instanceKubeconfig := KubernetesGetKubeconfigBytes(instanceName, clientset)
	if err != nil {
		return err
	}
	err, instanceClientset := KubernetesClientsetFromKubeconfigBytes(instanceKubeconfig)
	if err != nil {
		return err
	}

	//   wait for cluster and namespace availability
	restClient := instanceClientset.Discovery().RESTClient()
pollInstanceAPIServer:
	for true {
		deadline := time.Now().Add(time.Second * 3)
		ctx, _ := context.WithDeadline(context.TODO(), deadline)
		_, err := restClient.Get().AbsPath("/healthz").DoRaw(ctx)
		if err == nil {
			break pollInstanceAPIServer
		} else {
			log.Printf("err: %#v\n", err)
		}
		log.Printf("Instance '%v' not alive yet\n", instanceName)
		time.Sleep(time.Second * 5)
	}
	log.Printf("Instance '%v' alive\n", instanceName)

pollInstanceNamespace:
	for true {
		deadline := time.Now().Add(time.Second * 3)
		ctx, _ := context.WithDeadline(context.TODO(), deadline)
		ns, err := instanceClientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if err == nil && ns.ObjectMeta.Name == namespace {
			log.Printf("Found namespace '%v' on Instance '%v'\n", namespace, instanceName)
			break pollInstanceNamespace
		}
		log.Printf("Failed to find namespace '%v' on Instance '%v', %v\n", namespace, instanceName, err)
		time.Sleep(time.Second * 5)
	}

	// if cert doesn't exist locally
	if apierrors.IsNotFound(errLocalInstance) {
		err = nil
		log.Printf("Cert for Instance '%v' not found locally. Fetching from Instance\n", instanceName)
		//   get remote cert
		var instanceSecret *corev1.Secret
	pollInstanceSecretWildcardTLSCert:
		for true {
			err, instanceSecret = KubernetesGetInstanceWildcardTLSCert(clientset, instanceName)
			if apierrors.IsNotFound(err) {
				log.Printf("Secret 'letsencrypt-prod' is not found in Namespace 'powerdns' on Instance '%v' yet\n", instanceName)
				time.Sleep(time.Second * 5)
				continue
			}
			if instanceSecret.ObjectMeta.Name != "" {
				break pollInstanceSecretWildcardTLSCert
			}
			time.Sleep(time.Second * 5)
		}
		//   upsert remote cert locally
		err = KubernetesUpsertLocalInstanceWildcardTLSCert(clientset, instanceName, instanceSecret)
		if err != nil {
			log.Printf("%#v\n", err)
		}
	} else if err == nil {
		log.Printf("Cert for Instance '%v' found locally. Creating it in the Instance\n", instanceName)
		//   upsert local cert secret to remote
		err = KubernetesUpsertInstanceWildcardTLSCert(instanceClientset, instanceName, localSecret)
		if err != nil {
			log.Printf("%#v\n", err)
		}
	} else {
		log.Printf("%#v\n", err)
	}
	return err
}

// UpdateInstanceSpecIfEnvOverrides ...
// allow overrides from instance.Setup.Env to set fields in instance
// this way is a quick way to test new fields for new instances, but ideally these fields will be written by the client
func UpdateInstanceSpecIfEnvOverrides(instance InstanceSpec) InstanceSpec {
	instance.NodeSize = common.ReturnValueOrDefault(GetValueFromEnvSlice(instance.Setup.Env, "__SHARINGIO_PAIR_NODE_SIZE"), instance.NodeSize)
	instance.Setup.HumacsVersion = common.ReturnValueOrDefault(GetValueFromEnvSlice(instance.Setup.Env, "__SHARINGIO_PAIR_HUMACS_VERSION"), instance.Setup.HumacsVersion)
	instance.Setup.HumacsRepository = common.ReturnValueOrDefault(GetValueFromEnvSlice(instance.Setup.Env, "__SHARINGIO_PAIR_HUMACS_REPOSITORY"), instance.Setup.HumacsRepository)
	instance.Setup.Timezone = common.ReturnValueOrDefault(GetValueFromEnvSlice(instance.Setup.Env, "TZ"), instance.Setup.Timezone)
	instance.Setup.KubernetesVersion = common.ReturnValueOrDefault(GetValueFromEnvSlice(instance.Setup.Env, "__SHARINGIO_PAIR_KUBERNETES_VERSION"), instance.Setup.KubernetesVersion)
	return instance
}
