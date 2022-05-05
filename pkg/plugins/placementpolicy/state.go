package placementpolicy

import (
	"github.com/Azure/placement-policy-scheduler-plugins/pkg/plugins/placementpolicy/core"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

type stateData struct {
	name       string
	policy     *core.PolicyInfo
	nodeLabels map[string]string
}

func NewStateData(name string, info *core.PolicyInfo, nodeLabels map[string]string) framework.StateData {
	return &stateData{
		name:       name,
		policy:     info,
		nodeLabels: nodeLabels,
	}
}

func (d *stateData) Clone() framework.StateData {
	return d
}
