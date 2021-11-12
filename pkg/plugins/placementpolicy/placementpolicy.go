package placementpolicy

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync"

	"github.com/Azure/placement-policy-scheduler-plugins/apis/v1alpha1"
	ppclientset "github.com/Azure/placement-policy-scheduler-plugins/pkg/client/clientset/versioned"
	ppinformers "github.com/Azure/placement-policy-scheduler-plugins/pkg/client/informers/externalversions"
	"github.com/Azure/placement-policy-scheduler-plugins/pkg/plugins/placementpolicy/core"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// Plugin is a plugin that schedules pods on nodes based on
// PlacementPolicy custom resource.
type Plugin struct {
	sync.RWMutex
	frameworkHandler        framework.Handle
	ppMgr                   core.Manager
	placementPolicyPodInfos core.PlacementPolicyPodInfos
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
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	client := kubernetes.NewForConfigOrDie(cfg)
	ppClient := ppclientset.NewForConfigOrDie(cfg)
	ppInformerFactory := ppinformers.NewSharedInformerFactory(ppClient, 0)
	ppInformer := ppInformerFactory.Placementpolicy().V1alpha1().PlacementPolicies()

	ppMgr := core.NewPlacementPolicyManager(
		client,
		ppClient,
		handle.SnapshotSharedLister(),
		ppInformer,
		handle.SharedInformerFactory().Core().V1().Pods().Lister())

	plugin := &Plugin{
		frameworkHandler:        handle,
		ppMgr:                   ppMgr,
		placementPolicyPodInfos: core.NewPlacementPolicyPodInfos(),
	}

	ctx := context.Background()
	ppInformerFactory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), ppInformer.Informer().HasSynced) {
		err := fmt.Errorf("WaitForCacheSync failed")
		klog.ErrorS(err, "Cannot sync caches")
		return nil, err
	}

	return plugin, nil
}

// Name returns name of the plugin. It is used in logs, etc.
func (p *Plugin) Name() string {
	return Name
}

// Filter invoked at the filter extension point.
func (p *Plugin) Filter(ctx context.Context, cycleState *framework.CycleState, pod *corev1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	// filter is invoked in parallel for the single pod and all the nodes available in the scheduler.
	p.Lock()
	defer p.Unlock()
	if nodeInfo.Node() == nil {
		return framework.NewStatus(framework.Error, "node not found")
	}

	// cycle state is used to store the information about the processed pod and if it's already annotated.
	c, err := getCycleState(cycleState)
	if err != nil {
		return framework.NewStatus(framework.Error, err.Error())
	}

	// get the placement policy that matches pod
	pp, err := p.ppMgr.GetPlacementPolicyForPod(ctx, pod)
	if err != nil {
		return framework.NewStatus(framework.Error, fmt.Sprintf("failed to get placement policy for pod %s: %v", pod.Name, err))
	}
	// no placement policy that matches pod, then we skip filter plugin
	if pp == nil {
		klog.InfoS("no placement policy found for pod", "pod", pod.Name, "node", nodeInfo.Node().Name, "plugin", "filter", "action", "skip_filter")
		return nil
	}
	// skip filtering if the enforcement mode is best effort
	// only filter if the enforcement mode is strict
	if pp.Spec.EnforcementMode == v1alpha1.EnforcementModeBestEffort {
		klog.InfoS("placement policy enforcement mode is best effort", "pod", pod.Name, "node", nodeInfo.Node().Name, "plugin", "filter", "action", "skip_filter")
		return nil
	}

	nodeList, err := p.frameworkHandler.SnapshotSharedLister().NodeInfos().List()
	if err != nil {
		return framework.NewStatus(framework.Error, fmt.Sprintf("failed to get nodes in the cluster: %v", err))
	}

	// nodeWithMatchingLabels is a group of nodes that have the same labels as defined in the placement policy
	nodeWithMatchingLabels := groupNodesWithLabels(nodeList, pp.Spec.NodeSelector.MatchLabels)

	podList, err := p.ppMgr.GetPodsWithLabels(ctx, pp.Spec.PodSelector.MatchLabels)
	if err != nil {
		return framework.NewStatus(framework.Error, fmt.Sprintf("failed to get pods with labels: %v", err))
	}

	// podsOnNodeWithMatchingLabels is a group of pods with matching pod labels defined in placement policy
	// that are already on the nodes with matching labels or annotated to be on the nodes with matching node labels
	// by the placement policy scheduler plugin
	podsOnNodeWithMatchingLabels := len(groupPodsBasedOnNodePreference(podList, pod, nodeWithMatchingLabels))

	targetSize, err := intstr.GetScaledValueFromIntOrPercent(pp.Spec.Policy.TargetSize, len(podList), false)
	if err != nil {
		return framework.NewStatus(framework.Error, fmt.Sprintf("failed to get scaled value from int or percent: %v", err))
	}
	// if the action is mustnot, we'll use the inverse of the target size against total pods
	// to compute number of pods on nodes with matching labels
	if pp.Spec.Policy.Action == v1alpha1.ActionMustNot {
		targetSize = len(podList) - targetSize
	}

	klog.V(5).InfoS("total pods",
		"node", nodeInfo.Node().Name,
		"totalPods", len(podList),
		"podsOnNodeWithMatchingLabels", podsOnNodeWithMatchingLabels,
		"targetSize", targetSize, "plugin", "filter")

	var status *framework.Status
	var preferredNodeWithMatchingLabels bool
	_, isCurrentNodeInMatchingLabels := nodeWithMatchingLabels[nodeInfo.Node().Name]

	// the node in the current context belongs to the group of nodes with matching labels
	if isCurrentNodeInMatchingLabels {
		// if the number of pods on the node with matching labels is less than the target size, then we should prefer the node
		if podsOnNodeWithMatchingLabels < targetSize {
			klog.InfoS("matching nodes don't have enough pods", "pod", pod.Name, "node", nodeInfo.Node().Name, "plugin", "filter", "action", "skip_filter")
			preferredNodeWithMatchingLabels = true
		} else {
			// current node with matching labels already has enough pods, so filter out current node
			klog.InfoS("matching nodes have enough pods", "pod", pod.Name, "node", nodeInfo.Node().Name, "plugin", "filter", "action", "filter")
			status = framework.NewStatus(framework.Unschedulable, "")
			preferredNodeWithMatchingLabels = false
		}
	} else {
		// the node in the current context belongs to the group of nodes with non-matching labels
		if podsOnNodeWithMatchingLabels < targetSize {
			// filter this node as we prefer to schedule on nodes with matching labels first
			klog.InfoS("matching nodes don't have enough pods", "pod", pod.Name, "node", nodeInfo.Node().Name, "plugin", "filter", "action", "filter")
			status = framework.NewStatus(framework.Unschedulable, "")
			preferredNodeWithMatchingLabels = true
		} else {
			// don't filter this node as the nodes with matching labels have enough pods
			klog.InfoS("matching nodes have enough pods", "pod", pod.Name, "node", nodeInfo.Node().Name, "plugin", "filter", "action", "skip_filter")
			preferredNodeWithMatchingLabels = false
		}
	}

	// the filter extension point is invoked for the same pod with multiple nodes
	// the annotation needs to be set on the pod only once and the value should be the same for all nodes
	// we set the pod in the framework cycle state to avoid setting the annotation multiple times on the same pod
	ppPodInfo := c.info.Get(pod.UID)
	if ppPodInfo == nil || !ppPodInfo.PodAnnotated {
		klog.InfoS("annotating pod", "pod", pod.Name, "node", nodeInfo.Node().Name, "plugin", "filter")
		// annotate pod with placement policy
		pod, err = p.ppMgr.AnnotatePod(ctx, pod, pp, preferredNodeWithMatchingLabels)
		if err != nil {
			return framework.NewStatus(framework.Error, fmt.Sprintf("failed to annotate pod %s: %v", pod.Name, err))
		}
		c.info.Set(pod.UID, &core.PlacementPolicyPodInfo{PodName: pod.Name, PodAnnotated: true})
		cycleState.Write(PlacementPolicyPodStateKey, c)
	}

	return status
}

// Score invoked at the score extension point.
func (p *Plugin) Score(ctx context.Context, cycleState *framework.CycleState, pod *corev1.Pod, nodeName string) (int64, *framework.Status) {
	p.Lock()
	defer p.Unlock()

	c, err := getCycleState(cycleState)
	if err != nil {
		return 0, framework.NewStatus(framework.Error, err.Error())
	}

	nodeInfo, err := p.frameworkHandler.SnapshotSharedLister().NodeInfos().Get(nodeName)
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("getting node %q from Snapshot: %v", nodeName, err))
	}
	// get the placement policy that matches pod
	pp, err := p.ppMgr.GetPlacementPolicyForPod(ctx, pod)
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("failed to get placement policy for pod %s: %v", pod.Name, err))
	}
	// if placement policy enforcement mode is strict, then skip scoring
	if pp.Spec.EnforcementMode == v1alpha1.EnforcementModeStrict {
		return 0, nil
	}

	nodeList, err := p.frameworkHandler.SnapshotSharedLister().NodeInfos().List()
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("failed to get nodes in the cluster: %v", err))
	}

	// nodeWithMatchingLabels is a group of nodes that have the same labels as defined in the placement policy
	nodeWithMatchingLabels := groupNodesWithLabels(nodeList, pp.Spec.NodeSelector.MatchLabels)

	_, isCurrentNodeInMatchingLabels := nodeWithMatchingLabels[nodeInfo.Node().Name]

	ppPodInfo := c.info.Get(pod.UID)
	// the pod has already been in the scheduling process and we have the desired
	// node group in the cycle state
	if ppPodInfo != nil {
		score := 0
		if isCurrentNodeInMatchingLabels && ppPodInfo.PreferredNodeWithMatchingLabels ||
			!isCurrentNodeInMatchingLabels && !ppPodInfo.PreferredNodeWithMatchingLabels {
			score = 100
		}
		klog.InfoS("scheduling score", "pod", pod.Name, "node", nodeInfo.Node().Name, "score", score)
		return int64(score), nil
	}

	podList, err := p.ppMgr.GetPodsWithLabels(ctx, pp.Spec.PodSelector.MatchLabels)
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("failed to get pods with labels: %v", err))
	}

	// podsOnNodeWithMatchingLabels is a group of pods with matching pod labels defined in placement policy
	// that are already on the nodes with matching labels or annotated to be on the nodes with matching node labels
	// by the placement policy scheduler plugin
	podsOnNodeWithMatchingLabels := len(groupPodsBasedOnNodePreference(podList, pod, nodeWithMatchingLabels))

	targetSize, err := intstr.GetScaledValueFromIntOrPercent(pp.Spec.Policy.TargetSize, len(podList), false)
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("failed to get scaled value from int or percent: %v", err))
	}
	// if the action is mustnot, we'll use the inverse of the target size against total pods
	// to compute number of pods on nodes with matching labels
	if pp.Spec.Policy.Action == v1alpha1.ActionMustNot {
		targetSize = len(podList) - targetSize
	}

	klog.V(5).InfoS("total pods",
		"node", nodeInfo.Node().Name,
		"totalPods", len(podList),
		"podsOnNodeWithMatchingLabels", podsOnNodeWithMatchingLabels,
		"targetSize", targetSize, "plugin", "score")

	var score int64
	var preferredNodeWithMatchingLabels bool
	// the node in the current context belongs to the group of nodes with matching labels
	if isCurrentNodeInMatchingLabels {
		// if the number of pods on the node with matching labels is less than the target size, then we should prefer the node
		if podsOnNodeWithMatchingLabels < targetSize {
			klog.InfoS("matching nodes don't have enough pods", "pod", pod.Name, "node", nodeInfo.Node().Name, "plugin", "score")
			score = 100
			preferredNodeWithMatchingLabels = true
		} else {
			// current node with matching labels already has enough pods, so give it a score of 0
			klog.InfoS("matching nodes have enough pods", "pod", pod.Name, "node", nodeInfo.Node().Name, "plugin", "score")
			score = 0
			preferredNodeWithMatchingLabels = false
		}
	} else {
		// the node in the current context belongs to the group of nodes with non-matching labels
		if podsOnNodeWithMatchingLabels < targetSize {
			// score 0 for this node as we prefer to schedule on nodes with matching labels first
			klog.InfoS("matching nodes don't have enough pods", "pod", pod.Name, "node", nodeInfo.Node().Name, "plugin", "score")
			preferredNodeWithMatchingLabels = true
			score = 0
		} else {
			// nodes with matching labels have enough pods, so give it a score of 100
			klog.InfoS("matching nodes have enough pods", "pod", pod.Name, "node", nodeInfo.Node().Name, "plugin", "score")
			preferredNodeWithMatchingLabels = false
			score = 100
		}
	}

	if ppPodInfo == nil || !ppPodInfo.PodAnnotated {
		klog.InfoS("annotating pod", "pod", pod.Name, "node", nodeInfo.Node().Name, "plugin", "score")
		// annotate pod with placement policy
		pod, err = p.ppMgr.AnnotatePod(ctx, pod, pp, preferredNodeWithMatchingLabels)
		if err != nil {
			return 0, framework.NewStatus(framework.Error, fmt.Sprintf("failed to annotate pod %s: %v", pod.Name, err))
		}
		// update cycle state
		c.info.Set(pod.UID, &core.PlacementPolicyPodInfo{PodName: pod.Name, PodAnnotated: true, PreferredNodeWithMatchingLabels: preferredNodeWithMatchingLabels})
		cycleState.Write(PlacementPolicyPodStateKey, c)
	}

	return score, nil
}

// ScoreExtensions of the Score plugin.
func (p *Plugin) ScoreExtensions() framework.ScoreExtensions {
	return p
}

// NormalizeScore invoked after scoring all nodes.
func (p *Plugin) NormalizeScore(ctx context.Context, state *framework.CycleState, pod *corev1.Pod, scores framework.NodeScoreList) *framework.Status {
	// Find highest and lowest scores.
	var highest int64 = -math.MaxInt64
	var lowest int64 = math.MaxInt64
	for _, nodeScore := range scores {
		if nodeScore.Score > highest {
			highest = nodeScore.Score
		}
		if nodeScore.Score < lowest {
			lowest = nodeScore.Score
		}
	}

	// Transform the highest to lowest score range to fit the framework's min to max node score range.
	oldRange := highest - lowest
	newRange := framework.MaxNodeScore - framework.MinNodeScore
	for i, nodeScore := range scores {
		if oldRange == 0 {
			scores[i].Score = framework.MinNodeScore
		} else {
			scores[i].Score = ((nodeScore.Score - lowest) * newRange / oldRange) + framework.MinNodeScore
		}
	}

	klog.InfoS("normalized scores", "pod", pod.Name, "scores", scores)
	return nil
}

// checkHasLabels checks if the labels exist in the provided set
func checkHasLabels(l, wantLabels map[string]string) bool {
	if len(l) < len(wantLabels) {
		return false
	}

	for k, v := range wantLabels {
		if l[k] != v {
			return false
		}
	}
	return true
}

// groupNodesWithLabels groups all nodes that match the node labels defined in the placement policy
func groupNodesWithLabels(nodeList []*framework.NodeInfo, labels map[string]string) map[string]*framework.NodeInfo {
	// nodeWithMatchingLabels is a group of nodes that have the same labels as defined in the placement policy
	nodeWithMatchingLabels := make(map[string]*framework.NodeInfo)

	for _, node := range nodeList {
		if node.Node() == nil {
			continue
		}
		if checkHasLabels(node.Node().Labels, labels) {
			nodeWithMatchingLabels[node.Node().Name] = node
			continue
		}
	}

	return nodeWithMatchingLabels
}

// groupPodsBasedOnNodePreference groups all pods that match the node labels defined in the placement policy
func groupPodsBasedOnNodePreference(podList []*corev1.Pod, pod *corev1.Pod, nodeWithMatchingLabels map[string]*framework.NodeInfo) []*corev1.Pod {
	// podsOnNodeWithMatchingLabels is a group of pods with matching pod labels defined in placement policy
	// that are already on the nodes with matching labels or annotated to be on the nodes with matching node labels
	// by the placement policy scheduler plugin
	podsOnNodeWithMatchingLabels := []*corev1.Pod{}

	for _, p := range podList {
		// this scheduling cycle is for the current pod on a node, we should skip it
		if p.UID == pod.UID {
			continue
		}
		if p.Spec.NodeName != "" {
			if _, ok := nodeWithMatchingLabels[p.Spec.NodeName]; ok {
				podsOnNodeWithMatchingLabels = append(podsOnNodeWithMatchingLabels, p)
			}
			continue
		}
		// we could be at this point because of the following reasons:
		// 1. pod has not yet gone through scheduling process
		//    - in this case, the nodename and custom annotation set by our plugin is empty
		// 2. pod has gone through scheduling process but the nominated node hasn't been set yet
		//    - in this case, the nodename could be empty and we'll rely on the annotation to
		//		determine which group of nodes the pod is expected to land.
		ann := p.Annotations[v1alpha1.PlacementPolicyPreferenceAnnotationKey]
		// if the annotation is empty, we assume that the pod is still in the process of being scheduled
		if ann == "" {
			continue
		}
		preferredNodeWithMatchingLabels, err := strconv.ParseBool(ann)
		if err != nil {
			continue
		}
		// if the annotation is set to true, we count the pod as a pod on a node with matching labels
		if preferredNodeWithMatchingLabels {
			podsOnNodeWithMatchingLabels = append(podsOnNodeWithMatchingLabels, p)
			continue
		}
	}

	return podsOnNodeWithMatchingLabels
}

// Less is used to sort pods in the scheduling queue
func (cs *Plugin) Less(podInfo1, podInfo2 *framework.QueuedPodInfo) bool {
	return true
}
