package instances

import (
	"github.com/sharingio/pair/apps/cluster-api-manager/types"

	corev1 "k8s.io/api/core/v1"
	// networkingv1 "k8s.io/api/networking/v1"
	clusterAPIv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterAPIControlPlaneKubeadmv1alpha3 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
)

// Instance ...
// generic instance
// swagger:response instance
type Instance struct {
	Metadata types.JSONResponseMetadata `json:"metadata"`
	Spec     InstanceSpec               `json:"spec"`
	Status   InstanceStatus             `json:"status"`
}

// InstanceSpec ...
// specification for an instance
type InstanceSpec struct {
	Name                string             `json:"name"`
	Type                InstanceType       `json:"type"`
	Setup               types.SetupSpec    `json:"setup"`
	NodeSize            string             `json:"nodeSize"`
	KubernetesNodeCount int                `json:"kubernetesNodeCount"`
	Facility            string             `json:"facility"`
	NameScheme          InstanceNameScheme `json:"nameScheme"`
}

// InstanceResourceStatus ...
// various status fields for an instance
type InstanceResourceStatus struct {
	KubeadmControlPlane clusterAPIControlPlaneKubeadmv1alpha3.KubeadmControlPlaneStatus
	Cluster             clusterAPIv1alpha3.ClusterStatus
	HumacsPod           corev1.PodStatus
	MachineStatus       clusterAPIv1alpha3.MachineStatus
	PacketMachineUID    *string
}

// InstanceStatus ...
// status fields
type InstanceStatus struct {
	Phase     InstanceStatusPhase    `json:"phase"`
	Resources InstanceResourceStatus `json:"resources"`
}

// InstanceList ...
// generic instance list
// swagger:response instanceList
type InstanceList struct {
	Metadata types.JSONResponseMetadata `json:"metadata"`
	List     []InstanceSpec             `json:"list"`
}

// InstanceIngressList ...
// instance ingress list
// swagger:response instanceIngresses
type InstanceIngressList struct {
	Metadata types.JSONResponseMetadata `json:"metadata"`
	// TODO why does this line uncommented cause go-swagger to not work?
	// List     []networkingv1.Ingress     `json:"list"`
}

// InstanceKubeconfig ...
// kubeconfig response
// swagger:response instanceData
type InstanceKubeconfig struct {
	Metadata types.JSONResponseMetadata `json:"metadata"`
	Spec     string                     `json:"spec"`
}

// InstanceStatusPhase ...
// Instance phase status definitions
type InstanceStatusPhase string

// phases for instance status
const (
	InstanceStatusPhasePending      InstanceStatusPhase = "Pending"
	InstanceStatusPhaseProvisioning InstanceStatusPhase = "Provisioning"
	InstanceStatusPhaseProvisioned  InstanceStatusPhase = "Provisioned"
	InstanceStatusPhaseDeleting     InstanceStatusPhase = "Deleting"
)

// InstanceType ...
// types of valid instances
type InstanceType string

// instance types
const (
	InstanceTypeKubernetes InstanceType = "Kubernetes"
	InstanceTypePlain      InstanceType = "Plain"
)

// InstanceFilter ...
// fields to filter by when listing
type InstanceFilter struct {
	Username string       `json:"username"`
	Type     InstanceType `json:"type"`
}

// InstanceListOptions ...
// options for listing instances
type InstanceListOptions struct {
	Filter InstanceFilter `json:"filter"`
}

// InstanceNameScheme ...
// schemes for naming instances
type InstanceNameScheme string

// valid schemes for naming instances
const (
	InstanceNameSchemeSpecified            InstanceNameScheme = "Specified"
	InstanceNameSchemeUsername             InstanceNameScheme = "Username"
	InstanceNameSchemeGenerateFromUsername InstanceNameScheme = "GenerateFromUsername"
)

// InstanceCreateOptions ...
// options for creating instances
type InstanceCreateOptions struct {
	DryRun     bool
	NameScheme InstanceNameScheme
}
