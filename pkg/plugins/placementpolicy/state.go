package placementpolicy

import (
	"fmt"
	"strings"

	"github.com/Azure/placement-policy-scheduler-plugins/pkg/plugins/placementpolicy/core"

	"k8s.io/kubernetes/pkg/scheduler/framework"
)

const (
	// PlacementPolicyPodStateKey is the key to store in cycle state
	PlacementPolicyPodStateKey = "placementpolicy.k8s.io/pod-state"
)

type PodState struct {
	info core.PlacementPolicyPodInfos
}

func (p *PodState) Clone() framework.StateData {
	return &PodState{
		info: p.info.Clone(),
	}
}

func getCycleState(cycleState *framework.CycleState) (*PodState, error) {
	c, err := cycleState.Read(PlacementPolicyPodStateKey)
	if err == nil {
		if s, ok := c.(*PodState); ok {
			return s, nil
		}
		return nil, fmt.Errorf("unexpected item type. Expected: %T, Given: %T", &PodState{}, c)
	}
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return nil, err
	}
	return &PodState{
		info: core.NewPlacementPolicyPodInfos(),
	}, nil
}
