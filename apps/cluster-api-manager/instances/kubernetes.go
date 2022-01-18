package instances

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/sharingio/pair/apps/cluster-api-manager/common"
	"github.com/sharingio/pair/apps/cluster-api-manager/dns"

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
func KubernetesGet(name string, kubernetesClientset dynamic.Interface, clientset *kubernetes.Clientset) (instance Instance, err error) {
	targetNamespace := common.GetTargetNamespace()
	// manifests

	instance.Spec.Type = InstanceTypeKubernetes

	//   - newInstance.KubeadmControlPlane
	groupVersionResource := schema.GroupVersionResource{Version: "v1alpha3", Group: "controlplane.cluster.x-k8s.io", Resource: "kubeadmcontrolplanes"}
	item, err := kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Get(context.TODO(), fmt.Sprintf("%s-control-plane", name), metav1.GetOptions{})
	if err != nil {
		log.Printf("%#v\n", err)
	}
	var itemRestructuredKCP clusterAPIControlPlaneKubeadmv1alpha3.KubeadmControlPlane
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructuredKCP)
	if err != nil {
		return Instance{}, fmt.Errorf("Failed to restructure %T", itemRestructuredKCP)
	}
	if itemRestructuredKCP.ObjectMeta.Labels["io.sharing.pair"] != "instance" {
		log.Printf("Not using object %s/%T/%s - not an instance managed by sharingio/pair\n", targetNamespace, itemRestructuredKCP, itemRestructuredKCP.ObjectMeta.Name)
	} else {
		instance.Status.Resources.KubeadmControlPlane = itemRestructuredKCP.Status
	}

	//   - newInstance.Machine
	groupVersion := clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "cluster.x-k8s.io", Resource: "machines"}
	items, err := kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "cluster.x-k8s.io/cluster-name=" + name})
	if err != nil {
		log.Printf("%#v\n", err)
	} else if len(items.Items) > 0 {
		item = &items.Items[0]
		var itemRestructuredM clusterAPIv1alpha3.Machine
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructuredM)
		if err != nil {
			return Instance{}, fmt.Errorf("Failed to restructure %T", itemRestructuredM)
		}
		instance.Status.Resources.MachineStatus = itemRestructuredM.Status
	}

	//   - newInstance.PacketMachine
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "infrastructure.cluster.x-k8s.io", Resource: "packetmachines"}
	items, err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "cluster.x-k8s.io/cluster-name=" + name})
	if err != nil {
		log.Printf("%#v\n", err)
	} else if len(items.Items) > 0 {
		item = &items.Items[0]
		var itemRestructuredPM clusterAPIPacketv1alpha3.PacketMachine
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructuredPM)
		if err != nil {
			return Instance{}, fmt.Errorf("Failed to restructure %T", itemRestructuredPM)
		}
		var providerID string
		if itemRestructuredPM.Spec.ProviderID != nil {
			providerID = *itemRestructuredPM.Spec.ProviderID
		}
		providerIDSplit := strings.Split(providerID, "/")
		if len(providerIDSplit) == 3 {
			instance.Status.Resources.PacketMachineUID = &providerIDSplit[2]
		}
	}

	//   - newInstance.Cluster
	var itemRestructuredC clusterAPIv1alpha3.Cluster
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "cluster.x-k8s.io", Resource: "clusters"}
	item, err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		log.Printf("%#v\n", err)
		return instance, fmt.Errorf("Failed to get Cluster, %#v", err)
	}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructuredC)
	if err != nil {
		return Instance{}, fmt.Errorf("Failed to restructure %T", itemRestructuredC)
	}
	if itemRestructuredC.ObjectMeta.Labels["io.sharing.pair"] != "instance" {
		log.Printf("Not using object %s/%T/%s - not an instance managed by sharingio/pair\n", targetNamespace, itemRestructuredC, itemRestructuredC.ObjectMeta.Name)
	} else {
		instance.Status.Resources.Cluster = itemRestructuredC.Status
	}

	instance.Spec.Name = itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-name"]
	instance.Spec.NameScheme = InstanceNameScheme(itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-nameScheme"])
	instance.Spec.Setup.User = itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-setup-user"]
	instance.Spec.Setup.UserLowercase = strings.ToLower(instance.Spec.Setup.User)
	instance.Spec.NodeSize = itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-nodeSize"]
	instance.Spec.Facility = itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-facility"]
	kubernetesNodeCount, _ := strconv.Atoi(itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-kubernetesNodeCount"])
	instance.Spec.KubernetesNodeCount = kubernetesNodeCount
	instance.Spec.Setup.Guests = strings.Split(itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-setup-guests"], " ")
	instance.Spec.Setup.Repos = strings.Split(itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-setup-repos"], " ")
	instance.Spec.Setup.Timezone = itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-setup-timezone"]
	instance.Spec.Setup.Fullname = itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-setup-fullname"]
	instance.Spec.Setup.Email = itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-setup-email"]
	var env []map[string]string
	json.Unmarshal([]byte(itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-setup-env"]), &env)
	instance.Spec.Setup.Env = env
	instance.Spec.Setup.BaseDNSName = itemRestructuredC.ObjectMeta.Annotations["io.sharing.pair-spec-setup-baseDNSName"]

	var tmateSSH string
	tmateSSH, err = KubernetesGetTmateSSHSession(clientset, instance.Spec.Name, instance.Spec.Setup.UserLowercase)
	if err != nil {
		log.Printf("err: %#v\n", err.Error())
	}
	log.Printf("Instance '%v' tmate session: '%v'", instance.Spec.Name, tmateSSH)
	instance.Status.Phase = InstanceStatusPhaseProvisioning
	if instance.Status.Resources.Cluster.Phase == string(InstanceStatusPhaseDeleting) {
		instance.Status.Phase = InstanceStatusPhaseDeleting
	} else if firstSnippit := strings.Split(tmateSSH, " "); firstSnippit[0] == "ssh" {
		instance.Status.Phase = InstanceStatusPhaseProvisioned
	}
	log.Printf("Instance '%v' is at phase '%v'", instance.Spec.Name, instance.Status.Phase)

	return instance, nil
}

// KubernetesList ...
// list all Kubernetes instances
func KubernetesList(kubernetesClientset dynamic.Interface, clientset *kubernetes.Clientset, options InstanceListOptions) (instances []Instance, err error) {
	targetNamespace := common.GetTargetNamespace()

	groupVersionResource := schema.GroupVersionResource{Version: "v1alpha3", Group: "controlplane.cluster.x-k8s.io", Resource: "kubeadmcontrolplanes"}
	items, err := kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return instances, fmt.Errorf("Failed to list KubeadmControlPlane, %#v", err)
	}

	for _, item := range items.Items {
		var itemRestructured clusterAPIControlPlaneKubeadmv1alpha3.KubeadmControlPlane
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructured)
		if err != nil {
			return []Instance{}, fmt.Errorf("Failed to restructure %T", itemRestructured)
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
	items, err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return instances, fmt.Errorf("Failed to list Machine, %#v", err)
	}

	for _, item := range items.Items {
		var itemRestructured clusterAPIv1alpha3.Machine
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructured)
		if err != nil {
			return []Instance{}, fmt.Errorf("Failed to restructure %T", itemRestructured)
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
	items, err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return instances, fmt.Errorf("Failed to list PacketMachine, %#v", err)
	}

	for _, item := range items.Items {
		var itemRestructured clusterAPIPacketv1alpha3.PacketMachine
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructured)
		if err != nil {
			return []Instance{}, fmt.Errorf("Failed to restructure %T", itemRestructured)
		}
	instances2:
		for i := range instances {
			if instances[i].Spec.Name == itemRestructured.ObjectMeta.Labels["cluster.x-k8s.io/cluster-name"] {
				if itemRestructured.Spec.ProviderID == nil {
					continue instances2
				}
				var providerID string
				if itemRestructured.Spec.ProviderID != nil {
					providerID = *itemRestructured.Spec.ProviderID
				}
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
	items, err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return instances, fmt.Errorf("Failed to list Cluster, %#v", err)
	}

	for _, item := range items.Items {
		var itemRestructured clusterAPIv1alpha3.Cluster
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructured)
		if err != nil {
			return []Instance{}, fmt.Errorf("Failed to restructure %T", itemRestructured)
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
				instances[i].Spec.Setup.UserLowercase = strings.ToLower(instances[i].Spec.Setup.User)
				instances[i].Spec.Setup.Guests = strings.Split(itemRestructured.ObjectMeta.Annotations["io.sharing.pair-spec-setup-guests"], " ")
				instances[i].Spec.Setup.Repos = strings.Split(itemRestructured.ObjectMeta.Annotations["io.sharing.pair-spec-setup-repos"], " ")
				instances[i].Spec.Setup.Timezone = itemRestructured.ObjectMeta.Annotations["io.sharing.pair-spec-setup-timezone"]
				instances[i].Spec.Setup.Fullname = itemRestructured.ObjectMeta.Annotations["io.sharing.pair-spec-setup-fullname"]
				instances[i].Spec.Setup.Email = itemRestructured.ObjectMeta.Annotations["io.sharing.pair-spec-setup-email"]
				var env []map[string]string
				json.Unmarshal([]byte(itemRestructured.ObjectMeta.Annotations["io.sharing.pair-spec-setup-env"]), &env)
				instances[i].Spec.Setup.Env = env
				instances[i].Status.Resources.Cluster = itemRestructured.Status

				tmateSSH, err := KubernetesGetTmateSSHSession(clientset, instances[i].Spec.Name, instances[i].Spec.Setup.UserLowercase)
				if err != nil {
					log.Printf("err: %#v\n", err.Error())
				}
				instances[i].Status.Phase = InstanceStatusPhaseProvisioning
				if instances[i].Status.Resources.Cluster.Phase == string(InstanceStatusPhaseDeleting) {
					instances[i].Status.Phase = InstanceStatusPhaseDeleting
				} else if firstSnippit := strings.Split(tmateSSH, " "); firstSnippit[0] == "ssh" {
					instances[i].Status.Phase = InstanceStatusPhaseProvisioned
				}
				log.Printf("Instance '%v' is at phase '%v'", instances[i].Spec.Name, instances[i].Status.Phase)
				break instances3
			}
		}
	}
	return instances, nil
}

// KubernetesCreate ...
// create a Kubernetes Instance
func KubernetesCreate(instance InstanceSpec, dynamicClient dynamic.Interface, clientset *kubernetes.Clientset, options InstanceCreateOptions) (instanceCreated InstanceSpec, err error) {
	// generate name
	targetNamespace := common.GetTargetNamespace()
	if instance.KubernetesNodeCount > 3 {
		instance.KubernetesNodeCount = 3
	} else if instance.KubernetesNodeCount < 0 {
		instance.KubernetesNodeCount = 0
	}
	newInstance, err := KubernetesTemplateResources(instance, targetNamespace)
	if err != nil {
		return instanceCreated, err
	}
	instanceCreated = instance

	log.Printf("%#v\n", newInstance)

	if options.DryRun == true {
		log.Println("Exiting before create due to dry run")
		postKubeadmCommandYAML, _ := yaml.Marshal(newInstance.KubeadmControlPlane.Spec.KubeadmConfigSpec.PostKubeadmCommands)
		log.Printf("%v\n\n%#v", string(postKubeadmCommandYAML), instance)
		return instanceCreated, err
	}

	// manifests
	//   - newInstance.KubeadmControlPlane
	groupVersionResource := schema.GroupVersionResource{Version: "v1alpha3", Group: "controlplane.cluster.x-k8s.io", Resource: "kubeadmcontrolplanes"}
	log.Printf("%#v\n", groupVersionResource)
	asUnstructured, err := common.ObjectToUnstructured(newInstance.KubeadmControlPlane)
	asUnstructured.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1alpha3", Group: "controlplane.cluster.x-k8s.io", Kind: "KubeadmControlPlane"})
	_, err = dynamicClient.Resource(groupVersionResource).Namespace(targetNamespace).Create(context.TODO(), asUnstructured, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) != true {
		log.Printf("%#v\n", err)
		return instanceCreated, fmt.Errorf("Failed to create KubeadmControlPlane, %#v", err)
	}
	if apierrors.IsAlreadyExists(err) {
		log.Println("Already exists")
	}

	//   - newInstance.PacketMachineTemplate
	groupVersion := clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "infrastructure.cluster.x-k8s.io", Resource: "packetmachinetemplates"}
	log.Printf("%#v\n", groupVersionResource)
	asUnstructured, err = common.ObjectToUnstructured(newInstance.PacketMachineTemplate)
	asUnstructured.SetGroupVersionKind(schema.GroupVersionKind{Version: groupVersionResource.Version, Group: "infrastructure.cluster.x-k8s.io", Kind: "PacketMachineTemplate"})
	if err != nil {
		log.Printf("%#v\n", err)
		return instanceCreated, fmt.Errorf("Failed to unstructure PacketMachineTemplate, %#v", err)
	}
	_, err = dynamicClient.Resource(groupVersionResource).Namespace(targetNamespace).Create(context.TODO(), asUnstructured, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) != true {
		log.Printf("%#v\n", err)
		return instanceCreated, fmt.Errorf("Failed to create PacketMachineTemplate, %#v", err)
	}
	if apierrors.IsAlreadyExists(err) {
		log.Println("Already exists")
	}

	//   - newInstance.PacketCluster
	groupVersion = clusterAPIPacketv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "infrastructure.cluster.x-k8s.io", Resource: "packetclusters"}
	log.Printf("%#v\n", groupVersionResource)
	asUnstructured, err = common.ObjectToUnstructured(newInstance.PacketCluster)
	asUnstructured.SetGroupVersionKind(schema.GroupVersionKind{Version: groupVersionResource.Version, Group: "infrastructure.cluster.x-k8s.io", Kind: "PacketCluster"})
	if err != nil {
		log.Printf("%#v\n", err)
		return instanceCreated, fmt.Errorf("Failed to unstructure PacketCluster, %#v", err)
	}
	_, err = dynamicClient.Resource(groupVersionResource).Namespace(targetNamespace).Create(context.TODO(), asUnstructured, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) != true {
		log.Printf("%#v\n", err)
		return instanceCreated, fmt.Errorf("Failed to create PacketCluster, %#v", err)
	}
	if apierrors.IsAlreadyExists(err) {
		log.Println("Already exists")
	}

	//   - newInstance.Cluster
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "cluster.x-k8s.io", Resource: "clusters"}
	log.Printf("%#v\n", groupVersionResource)
	asUnstructured, err = common.ObjectToUnstructured(newInstance.Cluster)
	asUnstructured.SetGroupVersionKind(schema.GroupVersionKind{Version: groupVersionResource.Version, Group: "cluster.x-k8s.io", Kind: "Cluster"})
	if err != nil {
		log.Printf("%#v\n", err)
		return instanceCreated, fmt.Errorf("Failed to unstructure Cluster, %#v", err)
	}
	_, err = dynamicClient.Resource(groupVersionResource).Namespace(targetNamespace).Create(context.TODO(), asUnstructured, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) != true {
		log.Printf("%#v\n", err)
		return instanceCreated, fmt.Errorf("Failed to create Cluster, %#v", err)
	}
	if apierrors.IsAlreadyExists(err) {
		log.Println("Already exists")
	}

	//   - newInstance.MachineDeploymentWorker
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "cluster.x-k8s.io", Resource: "machinedeployments"}
	log.Printf("%#v\n", groupVersionResource)
	asUnstructured, err = common.ObjectToUnstructured(newInstance.MachineDeploymentWorker)
	asUnstructured.SetGroupVersionKind(schema.GroupVersionKind{Version: groupVersionResource.Version, Group: "cluster.x-k8s.io", Kind: "MachineDeployment"})
	if err != nil {
		log.Printf("%#v\n", err)
		return instanceCreated, fmt.Errorf("Failed to unstructure MachineDeployment, %#v", err)
	}
	_, err = dynamicClient.Resource(groupVersionResource).Namespace(targetNamespace).Create(context.TODO(), asUnstructured, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) != true {
		log.Printf("%#v\n", err)
		return instanceCreated, fmt.Errorf("Failed to create MachineDeployment, %#v", err)
	}
	if apierrors.IsAlreadyExists(err) {
		log.Println("Already exists")
	}

	//   - newInstance.KubeadmConfigTemplateWorker
	groupVersion = cabpkv1.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "bootstrap.cluster.x-k8s.io", Resource: "kubeadmconfigtemplates"}
	log.Printf("%#v\n", groupVersionResource)
	asUnstructured, err = common.ObjectToUnstructured(newInstance.KubeadmConfigTemplateWorker)
	asUnstructured.SetGroupVersionKind(schema.GroupVersionKind{Version: groupVersionResource.Version, Group: groupVersionResource.Group, Kind: "KubeadmConfigTemplate"})
	if err != nil {
		log.Printf("%#v\n", err)
		return instanceCreated, fmt.Errorf("Failed to unstructure KubeadmConfigTemplate, %#v", err)
	}
	_, err = dynamicClient.Resource(groupVersionResource).Namespace(targetNamespace).Create(context.TODO(), asUnstructured, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) != true {
		log.Printf("%#v\n", err)
		return instanceCreated, fmt.Errorf("Failed to create KubeadmConfigTemplate, %#v", err)
	}
	if apierrors.IsAlreadyExists(err) {
		log.Println("Already exists")
	}

	//   - newInstance.PacketMachineTemplateWorker
	groupVersion = clusterAPIv1alpha3.GroupVersion
	groupVersionResource = schema.GroupVersionResource{Version: groupVersion.Version, Group: "infrastructure.cluster.x-k8s.io", Resource: "packetmachinetemplates"}
	log.Printf("%#v\n", groupVersionResource)
	asUnstructured, err = common.ObjectToUnstructured(newInstance.PacketMachineTemplateWorker)
	asUnstructured.SetGroupVersionKind(schema.GroupVersionKind{Version: groupVersionResource.Version, Group: "infrastructure.cluster.x-k8s.io", Kind: "PacketMachineTemplate"})
	if err != nil {
		log.Printf("%#v\n", err)
		return instanceCreated, fmt.Errorf("Failed to unstructure PacketMachineTemplateWorker, %#v", err)
	}
	_, err = dynamicClient.Resource(groupVersionResource).Namespace(targetNamespace).Create(context.TODO(), asUnstructured, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) != true {
		log.Printf("%#v\n", err)
		return instanceCreated, fmt.Errorf("Failed to create PacketMachineTemplateWorker, %#v", err)
	}
	if apierrors.IsAlreadyExists(err) {
		log.Println("Already exists")
	}

	// TODO return the same creation fields (repos, guests, etc...)
	return instanceCreated, nil
}

// KubernetesUpdate ...
// update a Kubernetes instance
func KubernetesUpdate(instance InstanceSpec) (instanceUpdated InstanceSpec, err error) {
	return instanceUpdated, nil
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
	//   - newInstance.DNSEndpoint
	groupVersionResource = schema.GroupVersionResource{Version: "v1alpha1", Group: "externaldns.k8s.io", Resource: "dnsendpoints"}
	log.Printf("%#v\n", groupVersionResource)
	err = kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: "io.sharing.pair-spec-name=" + name})
	if err != nil && apierrors.IsNotFound(err) != true {
		log.Printf("%#v\n", err)
		return fmt.Errorf("Failed to delete DNSEndpoint, %#v", err)
	}
	err = nil

	return err
}

// KubernetesTemplateResources ...
// given an instance spec and namespace, return KubernetesCluster resources
func KubernetesTemplateResources(instance InstanceSpec, namespace string) (newInstance KubernetesCluster, err error) {
	instance.NodeSize = common.ReturnValueOrDefault(instance.NodeSize, GetInstanceDefaultNodeSize())
	instance.RegistryMirrors = common.GetInstanceContainerRegistryMirrors()
	instance.Setup.EnvironmentVersion = common.ReturnValueOrDefault(instance.Setup.EnvironmentVersion, GetEnvironmentVersion())
	instance.Setup.EnvironmentRepository = common.ReturnValueOrDefault(instance.Setup.EnvironmentRepository, GetEnvironmentRepository())
	instance.Setup.KubernetesVersion = common.ReturnValueOrDefault(instance.Setup.KubernetesVersion, GetKubernetesVersion())
	instance = UpdateInstanceSpecIfEnvOverrides(instance)

	var sshKeys []string
	for _, account := range append(instance.Setup.Guests, instance.Setup.User) {
		log.Printf("Fetching SSH key for '%v'\n", account)
		if account == "" {
			continue
		}
		githubSSHKeys, err := GetGitHubUserSSHKeys(account)
		if err != nil {
			log.Printf("Error getting SSH keys: %v\n", err.Error())
			sshKeys = []string{}
		}
		sshKeys = append(sshKeys, githubSSHKeys...)
	}
	instance.Setup.BaseDNSName = instance.Name + "." + common.GetBaseHost()
	instance.Setup.GuestsNamesFlat = strings.Join(instance.Setup.Guests, " ")
	tmpl, err := template.New(fmt.Sprintf("pair-instance-template-pre-%s-%v", instance.Name, time.Now().Unix())).Parse(`
cat << EOF >> /root/.sharing-io-pair-init.env
export KUBERNETES_VERSION={{ $.Setup.KubernetesVersion }}
export SHARINGIO_PAIR_INSTANCE_SETUP_USER="{{ $.Setup.User }}"
{{ if $.RegistryMirrors }}
export SHARINGIO_PAIR_INSTANCE_CONTAINER_REGISTRY_MIRRORS="{{ range $.RegistryMirrors }}{{ . }} {{ end }}"
{{ end }}

{{ range $key, $map := .Setup.Env }}
{{ range $mapkey, $mapvalue := $map }}
{{ if (and (eq $mapkey "SHARINGIO_REPO_BRANCH") (ne $mapvalue "")) }}
export SHARINGIO_REPO_BRANCH="{{ $mapvalue }}"
{{ end }}
{{ end }}
{{ end }}
EOF
. /root/.sharing-io-pair-init.env
`)
	if err != nil {
		log.Printf("%#v\n", err)
		return newInstance, fmt.Errorf("Error templating pair-instance-template-pre commands: %#v", err)
	}
	templatedBuffer := new(bytes.Buffer)
	err = tmpl.Execute(templatedBuffer, instance)
	if err != nil {
		log.Printf("%#v\n", err)
		return newInstance, fmt.Errorf("Error templating pair-instance-template-pre commands: %#v", err)
	}
	kubeadmPre2 := templatedBuffer.String()

	kubeadmPre5 := `
cd /root
git clone https://github.com/${SHARINGIO_PAIR_INSTANCE_SETUP_USER}/.sharing.io || \
  git clone https://github.com/sharingio/.sharing.io

if ! ( [ -f .sharing.io/cluster-api/preKubeadmCommands.sh ] || [ -f .sharing.io/cluster-api/postKubeadmCommands.sh ] ); then
  rm -rf .sharing.io
  git clone https://github.com/sharingio/.sharing.io
fi

if [ -z "${SHARINGIO_REPO_BRANCH}" ]; then
  (
    cd ~/.sharing.io
    git switch "${SHARINGIO_REPO_BRANCH}" || true
  )
fi

cd /root/.sharing.io/cluster-api

bash -x ./preKubeadmCommands.sh
`

	tmpl, err = template.New(fmt.Sprintf("packetcloudconfigsecret%s%v", instance.Name, time.Now().Unix())).Parse(`
cat << EOF >> /root/.sharing-io-pair-init.env
export EQUINIX_METAL_PROJECT={{ .PacketProjectID }}
EOF`)
	if err != nil {
		log.Printf("%#v\n", err)
		return newInstance, fmt.Errorf("Error templating packetcloudconfigsecret command: %#v", err)
	}
	templatedBuffer = new(bytes.Buffer)
	err = tmpl.Execute(templatedBuffer, map[string]interface{}{
		"PacketProjectID":      common.GetPacketProjectID(),
		"controlPlaneEndpoint": "{{ .controlPlaneEndpoint }}",
	})
	if err != nil {
		log.Printf("%#v\n", err.Error())
		return newInstance, fmt.Errorf("Error templating packetcloudconfigsecret command: %#v", err)
	}
	kubeadmPost1 := templatedBuffer.String()

	tmpl, err = template.New(fmt.Sprintf("pair-instance-template-post-%s-%v", instance.Name, time.Now().Unix())).Parse(`
cat << EOF >> /root/.sharing-io-pair-init.env
export SHARINGIO_PAIR_INSTANCE_NAME="{{ $.Name }}"
export SHARINGIO_PAIR_INSTANCE_SETUP_EMAIL="{{ $.Setup.Email }}"
export SHARINGIO_PAIR_INSTANCE_SETUP_USER="{{ $.Setup.User }}"
export SHARINGIO_PAIR_INSTANCE_SETUP_USERLOWERCASE="{{ $.Setup.UserLowercase }}"
export SHARINGIO_PAIR_INSTANCE_SETUP_GUESTS="{{ range $.Setup.Guests }}{{ . }} {{ end }}"
export SHARINGIO_PAIR_INSTANCE_SETUP_BASEDNSNAME="{{ $.Setup.BaseDNSName }}"
export SHARINGIO_PAIR_INSTANCE_ENVIRONMENT_REPOSITORY="{{ $.Setup.EnvironmentRepository }}"
export SHARINGIO_PAIR_INSTANCE_ENVIRONMENT_VERSION="{{ $.Setup.EnvironmentVersion }}"
export SHARINGIO_PAIR_INSTANCE_SETUP_TIMEZONE="{{ $.Setup.Timezone }}"
export SHARINGIO_PAIR_INSTANCE_SETUP_FULLNAME="{{ $.Setup.Fullname }}"
export SHARINGIO_PAIR_INSTANCE_SETUP_EMAIL="{{ $.Setup.Email }}"
export SHARINGIO_PAIR_INSTANCE_SETUP_GITHUBOAUTHTOKEN="{{ $.Setup.GitHubOAuthToken }}"
export SHARINGIO_PAIR_INSTANCE_SETUP_REPOS="{{ range $.Setup.Repos }}{{ . }} {{ end }}"
export SHARINGIO_PAIR_INSTANCE_SETUP_REPOS_EXPANDED="
        {{ range $.Setup.Repos }}- {{ . }}
        {{ end }}
"
export SHARINGIO_PAIR_INSTANCE_SETUP_ENV_EXPANDED="
      {{- if $.Setup.Env }}{{ range $index, $map := $.Setup.Env }}{{ range $key, $value := $map }}
            - name: \"{{ $key }}\"
              value: \"{{ $value }}\"       {{ end }}{{ end }}{{- end }}
"
EOF

. /root/.sharing-io-pair-init.env

cd /root/.sharing.io/cluster-api

bash -x ./postKubeadmCommands.sh
`)
	if err != nil {
		log.Printf("%#v\n", err)
		return newInstance, fmt.Errorf("Error templating pair-instance-template-post commands: %#v", err)
	}
	templatedBuffer = new(bytes.Buffer)
	err = tmpl.Execute(templatedBuffer, instance)
	if err != nil {
		log.Printf("%#v\n", err)
		return newInstance, fmt.Errorf("Error templating pair-instance-template-post commands: %#v", err)
	}
	kubeadmPost2 := templatedBuffer.String()

	templatedBuffer = nil
	tmpl = nil

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
						`set -x`,
						`
cat << EOF >> /root/.sharing-io-pair-init.env
export KUBERNETES_CONTROLPLANE_ENDPOINT={{ .controlPlaneEndpoint }}
export SHARINGIO_PAIR_INSTANCE_NODE_TYPE=control-plane
EOF`,
						kubeadmPre2,
						"apt-get -y update",
						"DEBIAN_FRONTEND=noninteractive apt-get install -y git",
						kubeadmPre5,
					},
					PostKubeadmCommands: []string{
						`set -x`,
						kubeadmPost1,
						kubeadmPost2,
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
				Replicas:    Int32ToInt32Pointer(int32(instance.KubernetesNodeCount)),
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
							`set -x`,
							`
cat << EOF >> /root/.sharing-io-pair-init.env
export SHARINGIO_PAIR_INSTANCE_NODE_TYPE=worker
EOF`,
							kubeadmPre2,
							"apt-get -y update",
							"DEBIAN_FRONTEND=noninteractive apt-get install -y git",
							kubeadmPre5,
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
						SshKeys:     sshKeys,
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
	newInstance = defaultKubernetesClusterConfig
	newInstance.KubeadmControlPlane.ObjectMeta.Name = instance.Name + "-control-plane"
	newInstance.KubeadmControlPlane.ObjectMeta.Namespace = namespace
	newInstance.KubeadmControlPlane.ObjectMeta.Annotations = map[string]string{}
	newInstance.KubeadmControlPlane.ObjectMeta.Annotations["io.sharing.pair-spec-name"] = instance.Name
	newInstance.KubeadmControlPlane.ObjectMeta.Annotations["io.sharing.pair-spec-setup-user"] = instance.Setup.User
	newInstance.KubeadmControlPlane.Spec.InfrastructureTemplate.Name = instance.Name + "-control-plane"

	newInstance.KubeadmConfigTemplateWorker.Spec.Template.Spec.PreKubeadmCommands[1] = `
export SHARINGIO_PAIR_INSTANCE_NODE_TYPE=worker
`

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
	newInstance.Cluster.ObjectMeta.Annotations["io.sharing.pair-spec-nameScheme"] = string(instance.NameScheme)
	newInstance.Cluster.ObjectMeta.Annotations["io.sharing.pair-spec-nodeSize"] = instance.NodeSize
	newInstance.Cluster.ObjectMeta.Annotations["io.sharing.pair-spec-facility"] = instance.Facility
	newInstance.Cluster.ObjectMeta.Annotations["io.sharing.pair-spec-kubernetesNodeCount"] = fmt.Sprintf("%v", instance.KubernetesNodeCount)
	newInstance.Cluster.ObjectMeta.Annotations["io.sharing.pair-spec-setup-noGitHubToken"] = fmt.Sprintf("%v", instance.Setup.GitHubOAuthToken == "")
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
		return newInstance, err
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

	return newInstance, nil
}

// KubernetesGetKubeconfigBytes ...
// given an instance name and clientset, return the instance's kubeconfig as bytes
func KubernetesGetKubeconfigBytes(name string, clientset *kubernetes.Clientset) (kubeconfigBytes []byte, err error) {
	targetNamespace := common.GetTargetNamespace()
	secret, err := clientset.CoreV1().Secrets(targetNamespace).Get(context.TODO(), fmt.Sprintf("%s-kubeconfig", name), metav1.GetOptions{})
	if err != nil {
		log.Printf("%#v\n", err)
		return []byte{}, fmt.Errorf("Failed to get Kubernetes cluster Kubeconfig; err: %#v", err)
	}
	kubeconfigBytes = secret.Data["value"]
	return kubeconfigBytes, nil
}

// KubernetesGetKubeconfigYAML ...
// given an instance name and clientset, return the instance's kubeconfig as YAML
func KubernetesGetKubeconfigYAML(name string, clientset *kubernetes.Clientset) (kubeconfig string, err error) {
	kubeconfigBytes, err := KubernetesGetKubeconfigBytes(name, clientset)
	if err != nil {
		return kubeconfig, err
	}
	return string(kubeconfigBytes), nil
}

// KubernetesDynamicGetKubeconfigBytes ...
// given an instance name and dynamic client, return the instance's kubeconfig as bytes
func KubernetesDynamicGetKubeconfigBytes(name string, kubernetesClientset dynamic.Interface) (kubeconfig []byte, err error) {
	targetNamespace := common.GetTargetNamespace()
	groupVersionResource := schema.GroupVersionResource{Version: "v1", Group: "", Resource: "secrets"}
	secret, err := kubernetesClientset.Resource(groupVersionResource).Namespace(targetNamespace).Get(context.TODO(), fmt.Sprintf("%s-kubeconfig", name), metav1.GetOptions{})
	if err != nil {
		log.Printf("%#v\n", err)
		return []byte{}, fmt.Errorf("Failed to get Kubernetes cluster Kubeconfig; err: %#v", err)
	}
	var itemRestructured corev1.Secret
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(secret.Object, &itemRestructured)
	if err != nil {
		log.Printf("%#v\n", err)
		return []byte{}, fmt.Errorf("Failed to restructure %T; err: %v", itemRestructured, err)
	}
	kubeconfigBytes := itemRestructured.Data["value"]
	return kubeconfigBytes, nil
}

// KubernetesGetKubeconfig ...
// given an instance name and clientset, return a kubeconfig for clientset
func KubernetesGetKubeconfig(name string, clientset *kubernetes.Clientset) (kubeconfig *clientcmdapi.Config, err error) {
	valueBytes, err := KubernetesGetKubeconfigBytes(name, clientset)
	kubeconfig, err = clientcmd.Load(valueBytes)
	return kubeconfig, err
}

// KubernetesExec ...
// exec a command in an Instance Kubernetes Pod
func KubernetesExec(clientset *kubernetes.Clientset, restConfig *rest.Config, options ExecOptions) (stdout string, stderr string, err error) {
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name("environment-0").
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
		return stdout, stderr, err
	}
	var stdoutBuffer, stderrBuffer bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  options.Stdin,
		Stdout: &stdoutBuffer,
		Stderr: &stderrBuffer,
		Tty:    options.TTY,
	})
	if err != nil {
		return stdoutBuffer.String(), stderrBuffer.String(), err
	}

	if options.PreserveWhitespace {
		return stdoutBuffer.String(), stderrBuffer.String(), nil
	}
	return strings.TrimSpace(stdoutBuffer.String()), strings.TrimSpace(stderrBuffer.String()), nil

	// https://github.com/kubernetes/kubectl/blob/e65caf964573fbf671c4648032da4b7df7c7eaf0/pkg/cmd/exec/exec.go#L357
}

// KubernetesGetTmateSSHSession ...
// given a clienset, instancename, and username, get the tmate SSH session for the Environment Pod
func KubernetesGetTmateSSHSession(clientset *kubernetes.Clientset, instanceName string, userName string) (output string, err error) {
	err = KubernetesGetInstanceAPIServerLiveness(clientset, instanceName)
	if err != nil {
		return "", err
	}
	err = KubernetesGetInstanceEnvironmentPodReadiness(clientset, instanceName, userName)
	if err != nil {
		return "", err
	}
	instanceKubeconfig, err := KubernetesGetKubeconfigBytes(instanceName, clientset)
	if err != nil {
		return "", err
	}
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(instanceKubeconfig)
	if err != nil {
		return "", err
	}
	instanceClientset, err := KubernetesClientsetFromKubeconfigBytes(instanceKubeconfig)
	if err != nil {
		return "", err
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
		ContainerName:      "environment",
		CaptureStderr:      true,
		CaptureStdout:      true,
		PreserveWhitespace: false,
		TTY:                false,
	}
	stdout, stderr, err := KubernetesExec(instanceClientset, restConfig, execOptions)
	if stderr != "" {
		return stdout, fmt.Errorf(stderr)
	}
	return stdout, nil
}

// KubernetesGetTmateWebSession ...
// given a clienset, instancename, and username, get the tmate web session for the Environment Pod
func KubernetesGetTmateWebSession(clientset *kubernetes.Clientset, instanceName string, userName string) (output string, err error) {
	err = KubernetesGetInstanceAPIServerLiveness(clientset, instanceName)
	if err != nil {
		return "", err
	}
	err = KubernetesGetInstanceEnvironmentPodReadiness(clientset, instanceName, userName)
	if err != nil {
		return "", err
	}
	instanceKubeconfig, err := KubernetesGetKubeconfigBytes(instanceName, clientset)
	if err != nil {
		return "", err
	}
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(instanceKubeconfig)
	if err != nil {
		return "", err
	}
	instanceClientset, err := KubernetesClientsetFromKubeconfigBytes(instanceKubeconfig)
	if err != nil {
		return "", err
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
		ContainerName:      "environment",
		CaptureStderr:      true,
		CaptureStdout:      true,
		PreserveWhitespace: false,
		TTY:                false,
	}
	stdout, stderr, err := KubernetesExec(instanceClientset, restConfig, execOptions)
	if stderr != "" {
		return stdout, fmt.Errorf(stderr)
	}
	return stdout, nil
}

// KubernetesGetInstanceIngresses ...
// given a clienset and instance name, return the Ingresses available on the instance
func KubernetesGetInstanceIngresses(clientset *kubernetes.Clientset, instanceName string) (ingresses *networkingv1.IngressList, err error) {
	instanceKubeconfig, err := KubernetesGetKubeconfigBytes(instanceName, clientset)
	if err != nil {
		return &networkingv1.IngressList{}, err
	}
	instanceClientset, err := KubernetesClientsetFromKubeconfigBytes(instanceKubeconfig)
	if err != nil {
		return &networkingv1.IngressList{}, err
	}

	ingresses, err = instanceClientset.NetworkingV1().Ingresses("").List(context.TODO(), metav1.ListOptions{})
	return ingresses, err
}

// KubernetesClientsetFromKubeconfigBytes ...
// given an kubeconfig as a slice of bytes return a clientset
func KubernetesClientsetFromKubeconfigBytes(kubeconfigBytes []byte) (clientset *kubernetes.Clientset, err error) {
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigBytes)
	if err != nil {
		return &kubernetes.Clientset{}, err
	}
	clientset, err = kubernetes.NewForConfig(restConfig)
	return clientset, err
}

// KubernetesWaitForInstanceKubeconfig ...
// given a local clientset and instance name, wait for the instance kubeconfig to populate locally
func KubernetesWaitForInstanceKubeconfig(clientset *kubernetes.Clientset, instanceName string) {
	targetNamespace := common.GetTargetNamespace()
	kubeconfigName := fmt.Sprintf("%s-kubeconfig", instanceName)
pollInstanceNamespace:
	for true {
		deadline := time.Now().Add(time.Second * 1)
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
	machinesDynamic, err := dynamicClient.Resource(groupVersionResource).Namespace(targetNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "cluster.x-k8s.io/cluster-name=" + name})
	if err != nil {
		log.Printf("%#v\n", err)
		return err
	}
	eventObjectBytes, _ := json.Marshal(machinesDynamic)
	var machines clusterAPIv1alpha3.MachineList
	json.Unmarshal(eventObjectBytes, &machines)
	if len(machines.Items) == 0 {
		log.Printf("no machines available yet with label selector 'cluster.x-k8s.io/cluster-name=%v'", name)
		return fmt.Errorf("no machines available yet with label selector 'cluster.x-k8s.io/cluster-name=%v'", name)
	}
	machine := machines.Items[0]
	if len(machine.Status.Addresses) < 1 {
		log.Println("error: machine has no IP addresses")
		return fmt.Errorf("machine has no IP addresses")
	}
	if machine.Status.Addresses[1].Address == "" {
		log.Println("error: machine address is empty")
		return fmt.Errorf("machine address is empty")
	}
	if govalidator.IsIPv4(machine.Status.Addresses[1].Address) == false {
		log.Printf("error '%v' is not a valid IPv4 address", machine.Status.Addresses[1].Address)
		return fmt.Errorf("error '%v' is not a valid IPv4 address", machine.Status.Addresses[1].Address)
	}

	// NOTE first IP doesn't work, as it's used for the cluster's API; instead we will use the second, which works
	ipAddress = machine.Status.Addresses[1].Address
	log.Println("machine IP available:", ipAddress)
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
// given an instance clientset and instance, return a TLS wildcard cert
func KubernetesGetInstanceWildcardTLSCert(clientset *kubernetes.Clientset, instance InstanceSpec) (secret *corev1.Secret, err error) {
	targetNamespace := instance.Setup.UserLowercase
	templatedSecretName := "letsencrypt-prod"
	instanceKubeconfig, err := KubernetesGetKubeconfigBytes(instance.Name, clientset)
	if err != nil {
		return &corev1.Secret{}, err
	}
	instanceClientset, err := KubernetesClientsetFromKubeconfigBytes(instanceKubeconfig)
	if err != nil {
		return &corev1.Secret{}, err
	}

	secret, err = instanceClientset.CoreV1().Secrets(targetNamespace).Get(context.TODO(), templatedSecretName, metav1.GetOptions{})
	return secret, err
}

// KubernetesGetLocalInstanceWildcardTLSCert ...
// given a local clientset and instance name, return the local TLS wildcard cert
func KubernetesGetLocalInstanceWildcardTLSCert(clientset *kubernetes.Clientset, username string) (secret *corev1.Secret, err error) {
	targetNamespace := common.GetTargetNamespace()
	templatedSecretName := fmt.Sprintf("%v-tls", username)

	secret, err = clientset.CoreV1().Secrets(targetNamespace).Get(context.TODO(), templatedSecretName, metav1.GetOptions{})
	if err != nil {
		return &corev1.Secret{}, err
	}
	if secret.ObjectMeta.Name == templatedSecretName {
		log.Printf("Found secret '%v' in namespace '%v'\n", templatedSecretName, targetNamespace)
	}
	return secret, nil
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
	log.Printf("Attempting to create a secret locally for '%v' in namespace '%v'\n", templatedSecretName, targetNamespace)
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
func KubernetesUpsertInstanceWildcardTLSCert(clientset *kubernetes.Clientset, instance InstanceSpec, secret *corev1.Secret) (err error) {
	targetNamespace := instance.Setup.UserLowercase
	templatedSecretName := "letsencrypt-prod"
	templatedSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: templatedSecretName,
			Labels: map[string]string{
				"io.sharing.pair": "instance",
			},
			Annotations: secret.ObjectMeta.Annotations,
		},
		Type: secret.Type,
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
func KubernetesAddCertToMachine(clientset *kubernetes.Clientset, dynamicClient dynamic.Interface, instance InstanceSpec) (err error) {
	if (instance.NameScheme != InstanceNameSchemeSpecified && instance.NameScheme != InstanceNameSchemeUsername) || instance.NameScheme == "" {
		log.Printf("Will not manage certs, due to unaccepted NameScheme '%v'", instance.NameScheme)
		return nil
	}
	instanceName := instance.Name
	// if cert secret for user name exists locally
	namespace := instance.Setup.UserLowercase
	log.Printf("Managing cert for Instance '%v'\n", instanceName)
	localSecret, errLocalInstance := KubernetesGetLocalInstanceWildcardTLSCert(clientset, instanceName)

	KubernetesWaitForInstanceKubeconfig(clientset, instanceName)

	instanceKubeconfig, err := KubernetesGetKubeconfigBytes(instanceName, clientset)
	if err != nil {
		return err
	}
	instanceClientset, err := KubernetesClientsetFromKubeconfigBytes(instanceKubeconfig)
	if err != nil {
		return err
	}

	//   wait for cluster and namespace availability
	err = KubernetesGetInstanceAPIServerLiveness(clientset, instanceName)
	if err != nil {
		return err
	}
	log.Printf("Instance '%v' alive\n", instanceName)

	deadline := time.Now().Add(time.Second * 1)
	ctx, _ := context.WithDeadline(context.TODO(), deadline)
	_, err = instanceClientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to find namespace '%v' on Instance '%v', %v", namespace, instanceName, err)
	}
	log.Printf("Found namespace '%v' on Instance '%v'\n", namespace, instanceName)

	// if cert doesn't exist locally
	if apierrors.IsNotFound(errLocalInstance) {
		err = nil
		log.Printf("Cert for Instance '%v' not found locally. Fetching from Instance\n", instanceName)
		//   get remote cert
		var instanceSecret *corev1.Secret
		instanceSecret, err = KubernetesGetInstanceWildcardTLSCert(clientset, instance)
		if apierrors.IsNotFound(err) || instanceSecret.ObjectMeta.Name == "" {
			return fmt.Errorf("secret 'letsencrypt-prod' is not found in Namespace '%v' on Instance '%v' yet", namespace, instanceName)
		}
		//   upsert remote cert locally
		err = KubernetesUpsertLocalInstanceWildcardTLSCert(clientset, instanceName, instanceSecret)
		log.Printf("err: %v\n", err)
	} else if err == nil {
		log.Printf("Cert for Instance '%v' found locally. Creating it in the Instance\n", instanceName)
		//   upsert local cert secret to remote
		err = KubernetesUpsertInstanceWildcardTLSCert(instanceClientset, instance, localSecret)
	}
	return err
}

// KubernetesGetInstanceAPIServerLiveness returns an error if the instance APIServer is not live or healthy
func KubernetesGetInstanceAPIServerLiveness(clientset *kubernetes.Clientset, instanceName string) error {
	instanceKubeconfig, err := KubernetesGetKubeconfigBytes(instanceName, clientset)
	if err != nil {
		return err
	}
	instanceClientset, err := KubernetesClientsetFromKubeconfigBytes(instanceKubeconfig)
	if err != nil {
		return err
	}

	restClient := instanceClientset.Discovery().RESTClient()
	deadline := time.Now().Add(time.Second * 1)
	ctx, _ := context.WithDeadline(context.TODO(), deadline)
	_, err = restClient.Get().AbsPath("/healthz").DoRaw(ctx)
	if err != nil {
		return fmt.Errorf("Instance '%v' not alive yet", instanceName)
	}
	return nil
}

// KubernetesGetInstanceEnvironmentPodReadiness returns an error given the unreadiness of the Environment Pod
func KubernetesGetInstanceEnvironmentPodReadiness(clientset *kubernetes.Clientset, instanceName string, userLowercase string) error {
	instanceKubeconfig, err := KubernetesGetKubeconfigBytes(instanceName, clientset)
	if err != nil {
		return err
	}
	instanceClientset, err := KubernetesClientsetFromKubeconfigBytes(instanceKubeconfig)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(time.Second * 1)
	ctx, _ := context.WithDeadline(context.TODO(), deadline)
	environmentPod, err := instanceClientset.CoreV1().Pods(userLowercase).Get(ctx, "environment-0", metav1.GetOptions{})
	if err != nil {
		log.Printf("%#v\n", err)
	}
	if environmentPod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("Environment Pod is not running for instance '%v'", instanceName)
	}
	return nil
}

// UpdateInstanceSpecIfEnvOverrides ...
// sets overrides from instance.Setup.Env to set fields in instance
// this way is a quick way to test new fields for new instances, but ideally these fields will be written by the client
func UpdateInstanceSpecIfEnvOverrides(instance InstanceSpec) InstanceSpec {
	instance.NodeSize = common.ReturnValueOrDefault(GetValueFromEnvSlice(instance.Setup.Env, "__SHARINGIO_PAIR_NODE_SIZE"), instance.NodeSize)
	instance.Setup.EnvironmentVersion = common.ReturnValueOrDefault(GetValueFromEnvSlice(instance.Setup.Env, "__SHARINGIO_PAIR_ENVIRONMENT_VERSION"), instance.Setup.EnvironmentVersion)
	instance.Setup.EnvironmentRepository = common.ReturnValueOrDefault(GetValueFromEnvSlice(instance.Setup.Env, "__SHARINGIO_PAIR_ENVIRONMENT_REPOSITORY"), instance.Setup.EnvironmentRepository)
	instance.Setup.KubernetesVersion = common.ReturnValueOrDefault(GetValueFromEnvSlice(instance.Setup.Env, "__SHARINGIO_PAIR_KUBERNETES_VERSION"), instance.Setup.KubernetesVersion)
	instance.Setup.Timezone = common.ReturnValueOrDefault(GetValueFromEnvSlice(instance.Setup.Env, "TZ"), instance.Setup.Timezone)
	return instance
}

// UpdateInstanceNodeWithProviderID ...
// sets the ProviderID field in the Node resources of the target cluster
func UpdateInstanceNodeWithProviderID(clientset *kubernetes.Clientset, dynamicClient dynamic.Interface, instanceName string) error {
	// get provider ID from PacketMachines
	targetNamespace := common.GetTargetNamespace()
	groupVersion := clusterAPIv1alpha3.GroupVersion
	groupVersionResource := schema.GroupVersionResource{Version: groupVersion.Version, Group: "infrastructure.cluster.x-k8s.io", Resource: "packetmachines"}
	items, err := dynamicClient.Resource(groupVersionResource).Namespace(targetNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "cluster.x-k8s.io/cluster-name=" + instanceName})

	instanceKubeconfig, err := KubernetesGetKubeconfigBytes(instanceName, clientset)
	if err != nil {
		return err
	}
	instanceClientset, err := KubernetesClientsetFromKubeconfigBytes(instanceKubeconfig)
	if err != nil {
		return err
	}
	// get all nodes in target cluster using it's kubeconfig, where they match on the name of the controlplane nodes
	for _, item := range items.Items {
		var itemRestructuredPM clusterAPIPacketv1alpha3.PacketMachine
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &itemRestructuredPM)
		if err != nil {
			return fmt.Errorf("failed to restructure %T", itemRestructuredPM)
		}
		node, err := instanceClientset.CoreV1().Nodes().Get(context.TODO(), itemRestructuredPM.ObjectMeta.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get Node '%v': %v", itemRestructuredPM.ObjectMeta.Name, err)
		}
		node.Spec.ProviderID = *itemRestructuredPM.Spec.ProviderID
		node.Spec.Taints = []corev1.Taint{}
		_, err = instanceClientset.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update Node %v: %v", itemRestructuredPM.ObjectMeta.Name, err)
		}
	}

	// update all nodes from list with provider ids
	return nil
}
