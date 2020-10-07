package instances

import (
	"github.com/sharingio/pair/src/cluster-api-manager/types"
)

type InstanceSpec struct {
	Name string `json:"name"`
	// either Kubernetes or Plain
	Type     InstanceType    `json:"type"`
	Setup    types.SetupSpec `json:"setup"`
	NodeSize string          `json:"nodeSize"`
	Facility string          `json:"facility"`
}

type InstanceStatus struct {
	Phase InstanceStatusPhase `json:"phase"`
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
