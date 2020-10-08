package instances

import (
	"github.com/sharingio/pair/src/cluster-api-manager/types"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	clusterAPIPacketv1alpha3 "sigs.k8s.io/cluster-api-provider-packet/api/v1alpha3"
	clusterAPIv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterAPIControlPlaneKubeadmv1alpha3 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
)

type Instance struct {
	Spec   InstanceSpec   `json:"spec"`
	Status InstanceStatus `json:"status"`
}

type InstanceSpec struct {
	Name string `json:"name"`
	// either Kubernetes or Plain
	Type     InstanceType    `json:"type"`
	Setup    types.SetupSpec `json:"setup"`
	NodeSize string          `json:"nodeSize"`
	Facility string          `json:"facility"`
}

type InstanceResourceStatus struct {
	KubeadmControlPlane     clusterAPIControlPlaneKubeadmv1alpha3.KubeadmControlPlaneStatus
	Cluster                 clusterAPIv1alpha3.ClusterStatus
	MachineDeploymentWorker clusterAPIv1alpha3.MachineDeploymentStatus
	PacketCluster           clusterAPIPacketv1alpha3.PacketClusterStatus
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
