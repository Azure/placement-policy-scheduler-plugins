package placementpolicy

import (
	"github.com/Azure/placement-policy-scheduler-plugins/apis/v1alpha1"

	"k8s.io/kubernetes/pkg/scheduler/framework"
)

type stateData struct {
	name string
	pp   *v1alpha1.PlacementPolicy
}

func NewStateData(name string, pp *v1alpha1.PlacementPolicy) framework.StateData {
	return &stateData{
		name: name,
		pp:   pp,
	}
}

func (d *stateData) Clone() framework.StateData {
	return d
}
