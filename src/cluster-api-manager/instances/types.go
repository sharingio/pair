package instances

import (
	"github.com/sharingio/pair/src/cluster-api-manager/types"
)

type InstanceSpec struct {
	Name     string          `json:","`
	// either Kubernetes or Plain
	Type     string          `json:"type"`
	Setup    types.SetupSpec `json:"setup"`
	NodeSize string          `json:"nodeSize"`
	Facility string          `json:"facility"`
}
