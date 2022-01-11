package core

import (
	"context"
	"sort"

	"github.com/Azure/placement-policy-scheduler-plugins/apis/v1alpha1"
	ppclientset "github.com/Azure/placement-policy-scheduler-plugins/pkg/client/clientset/versioned"
	ppinformers "github.com/Azure/placement-policy-scheduler-plugins/pkg/client/informers/externalversions/apis/v1alpha1"
	pplisters "github.com/Azure/placement-policy-scheduler-plugins/pkg/client/listers/apis/v1alpha1"
	"github.com/Azure/placement-policy-scheduler-plugins/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// Manager defines the interfaces for PlacementPolicy management.
type Manager interface {
	GetPlacementPolicyForPod(context.Context, *corev1.Pod) (*v1alpha1.PlacementPolicy, error)
	RemovePodFromPolicy(*corev1.Pod) error
	AddPodToPolicy(context.Context, *corev1.Pod, *v1alpha1.PlacementPolicy) (*PolicyInfo, error)
}

type PlacementPolicyManager struct {
	// client is a placementPolicy client
	ppClient ppclientset.Interface
	// podLister is pod lister
	podLister corelisters.PodLister
	// snapshotSharedLister is pod shared list
	snapshotSharedLister framework.SharedLister
	// ppLister is placementPolicy lister
	ppLister pplisters.PlacementPolicyLister
	// available policies by namespace
	policies PolicyInfos
}

func NewPlacementPolicyManager(
	ppClient ppclientset.Interface,
	snapshotSharedLister framework.SharedLister,
	ppInformer ppinformers.PlacementPolicyInformer,
	podLister corelisters.PodLister) *PlacementPolicyManager {
	return &PlacementPolicyManager{
		ppClient:             ppClient,
		snapshotSharedLister: snapshotSharedLister,
		ppLister:             ppInformer.Lister(),
		podLister:            podLister,
		policies:             NewPolicyInfos(),
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

			m.upsertPolicyInfo(matchingPolicy, true)
		}
	}
	return nil
}

func (m *PlacementPolicyManager) getInfoForPolicy(pp *v1alpha1.PlacementPolicy) (*PolicyInfo, bool) {
	ppNamespace := pp.Namespace
	ppName := pp.Name

	namespace, exists := m.policies[ppNamespace]

	if exists {
		policy, policyExists := namespace[ppName]
		if policyExists {
			return policy, true
		}
	}

	info := newPolicyInfo(ppNamespace, ppName, pp.Spec.Policy.Action, pp.Spec.Policy.TargetSize)
	return info, false
}

func (m *PlacementPolicyManager) AddPodToPolicy(ctx context.Context, pod *corev1.Pod, pp *v1alpha1.PlacementPolicy) (*PolicyInfo, error) {
	policy, exists := m.getInfoForPolicy(pp)
	addError := policy.addPodIfNotPresent(pod)
	m.upsertPolicyInfo(policy, exists)
	return policy, addError
}

func (m *PlacementPolicyManager) upsertPolicyInfo(policy *PolicyInfo, exists bool) {
	namespace := policy.Namespace
	name := policy.Name

	if exists {
		existing := m.policies[namespace][name]
		m.policies[namespace][name] = policy.merge(existing)
		return
	}

	namespacePolicies, namespaceExists := m.policies[namespace]
	if !namespaceExists {
		policies := make(map[string]*PolicyInfo)
		policies[name] = policy
		m.policies[namespace] = policies
		return
	}

	namespacePolicies[name] = policy
	m.policies[namespace] = namespacePolicies
}
