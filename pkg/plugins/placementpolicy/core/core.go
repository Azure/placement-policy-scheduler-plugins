package core

import (
	"context"
	"sort"

	"github.com/Azure/placement-policy-scheduler-plugins/apis/v1alpha1"
	ppinformers "github.com/Azure/placement-policy-scheduler-plugins/pkg/client/informers/externalversions/apis/v1alpha1"
	pplisters "github.com/Azure/placement-policy-scheduler-plugins/pkg/client/listers/apis/v1alpha1"
	"github.com/Azure/placement-policy-scheduler-plugins/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// Manager defines the interfaces for PlacementPolicy management.
type Manager interface {
	GetPlacementPolicyForPod(context.Context, *corev1.Pod) (*v1alpha1.PlacementPolicy, error)
	RemovePodFromPolicy(*corev1.Pod) error
	AddPodToPolicy(context.Context, *corev1.Pod, *v1alpha1.PlacementPolicy) (*PolicyInfo, error)
}

type PlacementPolicyManager struct {
	// ppLister is placementPolicy lister
	ppLister pplisters.PlacementPolicyLister
	// available policies by namespace
	policies PolicyInfos
}

func NewPlacementPolicyManager(
	ppInformer ppinformers.PlacementPolicyInformer) *PlacementPolicyManager {
	return &PlacementPolicyManager{
		ppLister: ppInformer.Lister(),
		policies: NewPolicyInfos(),
	}
}

// GetPlacementPolicyForPod returns the placement policy for the given pod
func (m *PlacementPolicyManager) GetPlacementPolicyForPod(ctx context.Context, pod *corev1.Pod) (*v1alpha1.PlacementPolicy, error) {
	ppList, err := m.ppLister.PlacementPolicies(pod.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}
	// filter the placement policy list based on the pod's labels
	ppList = m.filterPlacementPolicyList(ppList, pod)
	if len(ppList) == 0 {
		return nil, nil
	}
	if len(ppList) > 1 {
		// if there are multiple placement policies, sort them by weight and return the first one
		sort.Sort(sort.Reverse(ByWeight(ppList)))
	}

	return ppList[0], nil
}

func (m *PlacementPolicyManager) filterPlacementPolicyList(ppList []*v1alpha1.PlacementPolicy, pod *corev1.Pod) []*v1alpha1.PlacementPolicy {
	var filteredPPList []*v1alpha1.PlacementPolicy
	for _, pp := range ppList {
		labels := pp.Spec.PodSelector.MatchLabels
		if utils.HasMatchingLabels(pod.Labels, labels) {
			filteredPPList = append(filteredPPList, pp)
		}
	}
	return filteredPPList
}

func (m *PlacementPolicyManager) RemovePodFromPolicy(pod *corev1.Pod) error {
	key, keyError := framework.GetPodKey(pod)
	if keyError != nil {
		return keyError
	}

	podNamespace := pod.Namespace
	ppList := m.policies[podNamespace]

	if ppList != nil {
		var matchingPolicy *PolicyInfo

		for _, pp := range ppList {
			if pp.allQualifyingPods.Has(key) {
				matchingPolicy = pp
				break
			}
		}

		if matchingPolicy != nil {
			removeError := matchingPolicy.removePodIfPresent(pod)
			if removeError != nil {
				return removeError
			}

			m.updatePolicies(matchingPolicy)
		}
	}
	return nil
}

func (m *PlacementPolicyManager) getPolicyInfoForPlacementPolicy(pp *v1alpha1.PlacementPolicy) *PolicyInfo {
	ppNamespace := pp.Namespace
	ppName := pp.Name

	namespace, exists := m.policies[ppNamespace]

	if exists {
		policy, policyExists := namespace[ppName]
		if policyExists {
			return policy
		}
	}

	info := newPolicyInfo(ppNamespace, ppName, pp.Spec.Policy.Action, pp.Spec.Policy.TargetSize)
	return info
}

func (m *PlacementPolicyManager) AddPodToPolicy(ctx context.Context, pod *corev1.Pod, pp *v1alpha1.PlacementPolicy) (*PolicyInfo, error) {
	policy := m.getPolicyInfoForPlacementPolicy(pp)

	addError := policy.addPodIfNotPresent(pod)
	if addError != nil {
		return policy, addError
	}

	m.updatePolicies(policy)
	return policy, nil
}

func (m *PlacementPolicyManager) updatePolicies(policy *PolicyInfo) {
	namespace := policy.Namespace
	name := policy.Name

	namespacePolicies, namespaceExists := m.policies[namespace]

	if namespaceExists {
		existing, exists := namespacePolicies[name]
		if exists {
			qualifyingPodCount := len(policy.allQualifyingPods)
			if qualifyingPodCount > 0 {
				m.policies[namespace][name] = policy.merge(existing)
				return
			}

			// to ensure the in-memory collection of policies is kept reasonably sized, if a policy has 0 associated pods, it should be removed from the collection
			// since the lister for policies is always used to match to a pod, there is no opportunity for a pod to be matched with a deleted policy
			// on the flip side, this also means that changes to or deletion of a policy are handled here when a pod is added or removed versus attaching an event handler to the policy informer
			delete(m.policies[namespace], name)
			return
		}
	} else {
		m.policies[namespace] = make(map[string]*PolicyInfo)
	}
	m.policies[namespace][name] = policy
}
