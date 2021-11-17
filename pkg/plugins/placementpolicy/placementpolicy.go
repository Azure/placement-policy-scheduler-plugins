package placementpolicy

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// Plugin is a plugin that schedules pods in nodes based on
// PlacementPolicy custom resource.
type Plugin struct {
	frameworkHandler framework.Handle
}

const (
	// Name is the plugin name
	Name = "placementpolicy"
)

var _ framework.QueueSortPlugin = &Plugin{}
var _ framework.ScorePlugin = &Plugin{}
var _ framework.FilterPlugin = &Plugin{}

// New initializes and returns a new PlacementPolicy plugin.
func New(obj runtime.Object, handle framework.Handle) (framework.Plugin, error) {
	plugin := &Plugin{
		frameworkHandler: handle,
	}
	return plugin, nil
}

// Name returns name of the plugin. It is used in logs, etc.
func (p *Plugin) Name() string {
	return Name
}

// Score invoked at the score extension point.
func (p *Plugin) Score(ctx context.Context, state *framework.CycleState, pod *corev1.Pod, nodeName string) (int64, *framework.Status) {
	_, err := p.frameworkHandler.SnapshotSharedLister().NodeInfos().Get(nodeName)
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("getting node %q from Snapshot: %v", nodeName, err))
	}
	return 0, nil
}

// ScoreExtensions of the Score plugin.
func (p *Plugin) ScoreExtensions() framework.ScoreExtensions {
	return p
}

// NormalizeScore invoked after scoring all nodes.
func (p *Plugin) NormalizeScore(ctx context.Context, state *framework.CycleState, pod *corev1.Pod, scores framework.NodeScoreList) *framework.Status {
	return nil
}

func (p *Plugin) Filter(ctx context.Context, cycleState *framework.CycleState, pod *corev1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	return nil
}

// Less is used to sort pods in the scheduling queue
func (cs *Plugin) Less(podInfo1, podInfo2 *framework.QueuedPodInfo) bool {
	return true
}
