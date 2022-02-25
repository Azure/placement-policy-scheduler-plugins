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
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// Plugin is a plugin that schedules pods on nodes based on
// PlacementPolicy custom resource.
type Plugin struct {
	sync.RWMutex
	frameworkHandler framework.Handle
	ppMgr            core.Manager
}

const (
	// Name is the plugin name
	Name = "placementpolicy"
)

var _ framework.PreFilterPlugin = &Plugin{}
var _ framework.FilterPlugin = &Plugin{}
var _ framework.PreScorePlugin = &Plugin{}
var _ framework.ScorePlugin = &Plugin{}

// New initializes and returns a new PlacementPolicy plugin.
func New(obj runtime.Object, handle framework.Handle) (framework.Plugin, error) {

	ppClient := ppclientset.NewForConfigOrDie(handle.KubeConfig())
	ppInformerFactory := ppinformers.NewSharedInformerFactory(ppClient, 0)
	ppInformer := ppInformerFactory.Placementpolicy().V1alpha1().PlacementPolicies()

	ppMgr := core.NewPlacementPolicyManager(ppInformer)

	plugin := &Plugin{
		frameworkHandler: handle,
		ppMgr:            ppMgr,
	}

	ctx := context.Background()
	ppInformerFactory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), ppInformer.Informer().HasSynced) {
		err := fmt.Errorf("WaitForCacheSync failed")
		klog.ErrorS(err, "Cannot sync caches")
		return nil, err
	}

	podInformer := handle.SharedInformerFactory().Core().V1().Pods().Informer()
	podInformer.AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch t := obj.(type) {
				case *corev1.Pod:
					return true
				case cache.DeletedFinalStateUnknown:
					if _, ok := t.Obj.(*corev1.Pod); ok {
						return true
					}
					return false
				default:
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				DeleteFunc: plugin.RemovePodFromPolicy,
			},
		},
	)

	return plugin, nil
}

// Name returns name of the plugin. It is used in logs, etc.
func (p *Plugin) Name() string {
	return Name
}

// PreFilter performs the following.
// 1. Whether there is a placement policy for the pod.
// 2. Whether the placement policy is Strict.
// 3. Determines the node preference for the pod: node with labels matching placement policy or other
// 4. Annotate the pod with the node preference and the placement policy.
func (p *Plugin) PreFilter(ctx context.Context, state *framework.CycleState, pod *corev1.Pod) *framework.Status {
	// get the placement policy that matches pod
	pp, err := p.ppMgr.GetPlacementPolicyForPod(ctx, pod)
	if err != nil {
		return framework.NewStatus(framework.Error, fmt.Sprintf("failed to get placement policy for pod %s: %v", pod.Name, err))
	}
	// no placement policy that matches pod, then we skip filter plugin
	if pp == nil {
		klog.InfoS("no placement policy found for pod", "pod", pod.Name)
		return framework.NewStatus(framework.Success, "")
	}

	policy := p.ppMgr.GetPolicyInfo(pp)

	// skip filtering if the enforcement mode is best effort
	// only filter if the enforcement mode is strict
	if pp.Spec.EnforcementMode == v1alpha1.EnforcementModeBestEffort {
		matchedPolicy, err := p.ppMgr.MatchPod(ctx, pod, policy)
		if err != nil {
			return framework.NewStatus(framework.Error, fmt.Sprintf("failed to match pod %s with placement policy %s: %v", pod.Name, pp.Name, err))
		}
		state.Write(p.getPreFilterStateKey(), NewStateData(pod.Name, pp, matchedPolicy, Matched))
		return framework.NewStatus(framework.Success, "")
	}

	updatedPolicy, err := p.ppMgr.AddPod(ctx, pod, policy)

	if err != nil {
		return framework.NewStatus(framework.Error, fmt.Sprintf("failed to associate pod %s with placement policy %s: %v", pod.Name, pp.Name, err))
	}

	state.Write(p.getPreFilterStateKey(), NewStateData(pod.Name, pp, updatedPolicy, Added))
	return framework.NewStatus(framework.Success, "")
}

// PreFilterExtensions returns a PreFilterExtensions interface if the plugin implements one.
func (p *Plugin) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}

// Filter invoked at the filter extension point.
func (p *Plugin) Filter(ctx context.Context, state *framework.CycleState, pod *corev1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	if nodeInfo.Node() == nil {
		return framework.NewStatus(framework.Error, "node not found")
	}

	data, err := state.Read(p.getPreFilterStateKey())
	if err != nil {
		// if there is no data in state for the pod, then we should skip filter plugin
		// as there could be no placement policy for the pod
		if err == framework.ErrNotFound {
			return framework.NewStatus(framework.Success, "")
		}
		return framework.NewStatus(framework.Error, fmt.Sprintf("failed to read state: %v", err))
	}
	d, ok := data.(*stateData)
	if !ok {
		return framework.NewStatus(framework.Error, "failed to cast state data")
	}

	// if only "matched" in PreFilter, still a success, so continue
	if d.status == Matched {
		state.Write(p.getFilterStateKey(), d)
		return framework.NewStatus(framework.Success, "")
	}

	node := nodeInfo.Node()
	// nodeMatchesLabels is set to true if the node in the current context matches the node selector labels
	// defined in the placement policy chosen for the pod.
	nodeMatchesLabels := checkHasLabels(node.Labels, d.nodeLabels)

	podKey, err := framework.GetPodKey(pod)
	if keyError != nil {
		return framework.NewStatus(framework.Error, "pod key not found")
	}

	policyManagesPod := d.policy.PodIsManagedByPolicy(podKey)

	// if the node preference annotation on the pod matches the node group in the current context, then don't filter the node
	if nodeMatchesLabels && policyManagesPod ||
		!nodeMatchesLabels && !policyManagesPod {
		state.Write(p.getFilterStateKey(), d)
		return framework.NewStatus(framework.Success, "")
	}

	klog.InfoS("filtering node", "node", node.Name, "pod", pod.Name)
	return framework.NewStatus(framework.Unschedulable, "")
}

// PreScore performs the following.
// 1. Whether there is a placement policy for the pod.
// 2. Whether the placement policy is BestEffort.
// 3. Determines the node preference for the pod: node with labels matching placement policy or other
// 4. Annotate the pod with the node preference and the placement policy.
func (p *Plugin) PreScore(ctx context.Context, state *framework.CycleState, pod *corev1.Pod, nodes []*corev1.Node) *framework.Status {
	data, err := state.Read(p.getFilterStateKey())
	if err != nil {
		// if there is no data in state for the pod, then we should skip prescore plugin
		// as there could be no placement policy for the pod
		if err == framework.ErrNotFound {
			return framework.NewStatus(framework.Success, "")
		}
		return framework.NewStatus(framework.Error, fmt.Sprintf("failed to read state: %v", err))
	}
	d, ok := data.(*stateData)
	if !ok {
		return framework.NewStatus(framework.Error, "failed to cast state data")
	}

	//if pod has already been added to the policy in (Pre)Filter, don't need to do anything with scoring
	if d.status == Added {
		return framework.NewStatus(framework.Success, "")
	}

	policy, err := p.ppMgr.AddPod(ctx, pod, d.policy)

	if err != nil {
		return framework.NewStatus(framework.Error, fmt.Sprintf("failed to associate pod %s with placement policy %s: %v", pod.Name, d.policy.Name, err))
	}

	state.Write(p.getPreScoreStateKey(), ModifedStateData(pod.Name, policy, d.nodeLabels, Added))
	return framework.NewStatus(framework.Success, "")
}

// Score invoked at the score extension point.
func (p *Plugin) Score(ctx context.Context, state *framework.CycleState, pod *corev1.Pod, nodeName string) (int64, *framework.Status) {
	data, err := state.Read(p.getPreScoreStateKey())
	if err != nil {
		// if there is no data in state for the pod, then we should skip score plugin
		if err == framework.ErrNotFound {
			return 0, nil
		}
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("failed to read state: %v", err))
	}
	d, ok := data.(*stateData)
	if !ok {
		return 0, framework.NewStatus(framework.Error, "failed to cast state data")
	}

	//this should never happen since the status is set when saving the prescore state
	if d.status != Added {
		return 0, nil
	}

	nodeInfo, err := p.frameworkHandler.SnapshotSharedLister().NodeInfos().Get(nodeName)
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("getting node %q from Snapshot: %v", nodeName, err))
	}
	node := nodeInfo.Node()
	// nodeMatchesLabels is set to true if the node in the current context matches the node selector labels
	// defined in the placement policy chosen for the pod.
	nodeMatchesLabels := checkHasLabels(node.Labels, d.nodeLabels)

	podKey, keyError := framework.GetPodKey(pod)
	if keyError != nil {
		return 0, framework.NewStatus(framework.Error, "pod key not found")
	}
	policyManagesPod := d.policy.PodIsManagedByPolicy(podKey)

	// if the node preference annotation on the pod matches the node group in the current context, then don't filter the node
	if nodeMatchesLabels && policyManagesPod ||
		!nodeMatchesLabels && !policyManagesPod {
		return 100, nil
	}

	return 0, nil
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
	return framework.NewStatus(framework.Success, "")
}

func (p *Plugin) getPreFilterStateKey() framework.StateKey {
	return framework.StateKey(fmt.Sprintf("Prefilter-%v", p.Name()))
}

func (p *Plugin) getFilterStateKey() framework.StateKey {
	return framework.StateKey(fmt.Sprintf("Filter-%v", p.Name()))
}

func (p *Plugin) getPreScoreStateKey() framework.StateKey {
	return framework.StateKey(fmt.Sprintf("Prescore-%v", p.Name()))
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
func groupNodesWithLabels(nodeList []*corev1.Node, labels map[string]string) map[string]*corev1.Node {
	// nodeWithMatchingLabels is a group of nodes that have the same labels as defined in the placement policy
	nodeWithMatchingLabels := make(map[string]*corev1.Node)

	for _, node := range nodeList {
		if checkHasLabels(node.Labels, labels) {
			nodeWithMatchingLabels[node.Name] = node
			continue
		}
	}

	return nodeWithMatchingLabels
}

// groupPodsBasedOnNodePreference groups all pods that match the node labels defined in the placement policy
func groupPodsBasedOnNodePreference(podList []*corev1.Pod, pod *corev1.Pod, nodeWithMatchingLabels map[string]*corev1.Node) []*corev1.Pod {
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

func (p *Plugin) RemovePodFromPolicy(obj interface{}) {
	pod := obj.(*corev1.Pod)

	p.Lock()
	defer p.Unlock()

	removeError := p.ppMgr.RemovePod(pod)
	if removeError != nil {
		klog.ErrorS(removeError, "error removing pod from placement policy")
	}
}
