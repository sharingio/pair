package instances

import (
	"github.com/sharingio/pair/types"

	corev1 "k8s.io/api/core/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	clusterAPIv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterAPIControlPlaneKubeadmv1alpha3 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
)

type Instance struct {
	Spec   InstanceSpec   `json:"spec"`
	Status InstanceStatus `json:"status"`
}

type InstanceSpec struct {
	Name     string          `json:"name"`
	Type     InstanceType    `json:"type"`
	Setup    types.SetupSpec `json:"setup"`
	NodeSize string          `json:"nodeSize"`
	Facility string          `json:"facility"`
}

type InstanceResourceStatus struct {
	KubeadmControlPlane clusterAPIControlPlaneKubeadmv1alpha3.KubeadmControlPlaneStatus
	Cluster             clusterAPIv1alpha3.ClusterStatus
	HumacsPod           corev1.PodStatus
	MachineStatus       clusterAPIv1alpha3.MachineStatus
	PacketMachineUID    *string
}

type InstanceStatus struct {
	Phase     InstanceStatusPhase    `json:"phase"`
	Resources InstanceResourceStatus `json:"resources"`
}

type InstanceStatusPhase string

const (
	InstanceStatusPhasePending      InstanceStatusPhase = "Pending"
	InstanceStatusPhaseProvisioning InstanceStatusPhase = "Provisioning"
	InstanceStatusPhaseProvisioned  InstanceStatusPhase = "Provisioned"
	InstanceStatusPhaseDeleting     InstanceStatusPhase = "Deleting"
)

type InstanceType string

const (
	InstanceTypeKubernetes InstanceType = "Kubernetes"
	InstanceTypePlain      InstanceType = "Plain"
)

type InstanceFilter struct {
	Username string `json:"username"`
}

type InstanceListOptions struct {
	Filter InstanceFilter `json:"filter"`
}

type InstanceAccess struct {
	Kubeconfig  clientcmdapi.Config `json:"kubeconfig"`
	TmateString string              `json:"tmateString"`
}

type InstanceCreateOptions struct {
	DryRun bool
}
