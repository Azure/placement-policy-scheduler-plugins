package placementpolicy

import (
	"github.com/Azure/placement-policy-scheduler-plugins/apis/v1alpha1"
	"github.com/Azure/placement-policy-scheduler-plugins/pkg/plugins/placementpolicy/core"

	"k8s.io/kubernetes/pkg/scheduler/framework"
)

type stateData struct {
	name string
	pp   *v1alpha1.PlacementPolicy
	info *core.PolicyInfo
}

func NewStateData(name string, pp *v1alpha1.PlacementPolicy, info *core.PolicyInfo) framework.StateData {
	return &stateData{
		name: name,
		pp:   pp,
		info: info,
	}
}

func (d *stateData) Clone() framework.StateData {
	return d
}
