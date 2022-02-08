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
	GetPolicyInfo(*v1alpha1.PlacementPolicy) *PolicyInfo
	MatchPod(context.Context, *corev1.Pod, *PolicyInfo) (*PolicyInfo, error)
	AddPod(context.Context, *corev1.Pod, *PolicyInfo) (*PolicyInfo, error)
	RemovePod(*corev1.Pod) error
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

func (m *PlacementPolicyManager) GetPolicyInfo(pp *v1alpha1.PlacementPolicy) *PolicyInfo {
	policyNamespace := pp.Namespace
	policyName := pp.Name
	corePolicy := pp.Spec.Policy

	namespacePolicies, namespaceExists := m.policies[policyNamespace]

	if namespaceExists {
		policy, policyExists := namespacePolicies[policyName]

		if policyExists {
			return policy
		}

		created := newPolicyInfo(policyNamespace, policyName, corePolicy.Action, corePolicy.TargetSize)
		m.policies[policyNamespace][policyName] = created
		return created
	}

	m.policies[policyNamespace] = make(map[string]*PolicyInfo)
	created := newPolicyInfo(policyNamespace, policyName, corePolicy.Action, corePolicy.TargetSize)
	m.policies[policyNamespace][policyName] = created
	return created
}

type PodAction int16

const (
	Match PodAction = iota
	Add
	Remove
)

func (m *PlacementPolicyManager) MatchPod(ctx context.Context, pod *corev1.Pod, policy *PolicyInfo) (*PolicyInfo, error) {
	err := policy.addMatch(pod)
	if matchError != nil {
		return nil, matchError
	}

	m.updatePolicies(policy, Match)
	return policy, nil
}

func (m *PlacementPolicyManager) AddPod(ctx context.Context, pod *corev1.Pod, policy *PolicyInfo) (*PolicyInfo, error) {
	addError := policy.addPodIfNotPresent(pod)
	if addError != nil {
		return nil, addError
	}

	m.updatePolicies(policy, Add)
	return policy, nil
}

func (m *PlacementPolicyManager) RemovePod(pod *corev1.Pod) error {
	key, keyError := framework.GetPodKey(pod)
	if keyError != nil {
		return keyError
	}

	podNamespace := pod.Namespace
	ppList := m.policies[podNamespace]

	if ppList != nil {
		var matchingPolicy *PolicyInfo

		for _, pp := range ppList {
			if pp.PodQualifiesForPolicy(key) {
				matchingPolicy = pp
				break
			}
		}

		if matchingPolicy != nil {
			removeError := matchingPolicy.removePodIfPresent(pod)
			if removeError != nil {
				return removeError
			}

			m.updatePolicies(matchingPolicy, Remove)
		}
	}
	return nil
}

func (m *PlacementPolicyManager) updatePolicies(policy *PolicyInfo, act PodAction) {
	namespace := policy.Namespace
	name := policy.Name

	if act == Remove {
		qualifyingCount := len(policy.qualifiedPods)

		if qualifyingCount == 0 {
			delete(m.policies[namespace], name)
			return
		}
	}

	existing := m.policies[namespace][name]
	m.policies[namespace][name] = policy.merge(existing)
}
