package placementpolicy

import (
	"github.com/Azure/placement-policy-scheduler-plugins/apis/v1alpha1"
	"github.com/Azure/placement-policy-scheduler-plugins/pkg/plugins/placementpolicy/core"

	"k8s.io/kubernetes/pkg/scheduler/framework"
)

type PodPolicyStatus int16

const (
	Matched PodPolicyStatus = iota
	Added
	NoPolicy
)

type stateData struct {
	name       string
	policy     *core.PolicyInfo
	nodeLabels map[string]string
	status     PodPolicyStatus
}

func NewStateData(name string, pp *v1alpha1.PlacementPolicy, info *core.PolicyInfo, status PodPolicyStatus) framework.StateData {
	return &stateData{
		name:       name,
		policy:     info,
		nodeLabels: pp.Spec.NodeSelector.MatchLabels,
		status:     status,
	}
}

func ModifiedStateData(name string, info *core.PolicyInfo, nodeLabels map[string]string, status PodPolicyStatus) framework.StateData {
	return &stateData{
		name:       name,
		policy:     info,
		nodeLabels: nodeLabels,
		status:     status,
	}
}

func EmptyStateData(name string) framework.StateData {
	return &stateData{
		name:       name,
		nodeLabels: map[string]string{},
		status:     NoPolicy,
	}
}

func (d *stateData) Clone() framework.StateData {
	return d
}
