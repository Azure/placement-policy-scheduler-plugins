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

// GetPolicyInfo returns the PolicyInfo held in-memory for the given placement policy
func (m *PlacementPolicyManager) GetPolicyInfo(pp *v1alpha1.PlacementPolicy) *PolicyInfo {
	policyNamespace := pp.Namespace
	policyName := pp.Name
	corePolicy := pp.Spec.Policy
	action := corePolicy.Action
	targetSize := corePolicy.TargetSize

	return m.policies.GetPolicy(policyNamespace, policyName, action, targetSize)
}

type PodAction int16

const (
	Match PodAction = iota
	Add
	Remove
)

// AddPod adds the provided pod to the provided PolicyInfo and calculates whether target was met
func (m *PlacementPolicyManager) AddPod(ctx context.Context, pod *corev1.Pod, policy *PolicyInfo) (*PolicyInfo, error) {
	err := policy.addPodIfNotPresent(pod)
	if err != nil {
		return nil, err
	}

	m.updatePolicies(policy, Add)
	return policy, nil
}

// RemovePod removes the provided pod from PolicyInfo held in-memory and re-calculates policy's status (if applicable)
func (m *PlacementPolicyManager) RemovePod(pod *corev1.Pod) error {
	key, err := framework.GetPodKey(pod)
	if err != nil {
		return err
	}

	podNamespace := pod.Namespace
	ppList := m.policies.GetPoliciesByNamespace(podNamespace)

	if ppList == nil {
		return nil
	}

	var matchingPolicy *PolicyInfo

	for _, pp := range ppList {
		if pp.PodQualifiesForPolicy(key) {
			matchingPolicy = pp
			break
		}
	}

	if matchingPolicy != nil {
		err := matchingPolicy.removePodIfPresent(pod)
		if err != nil {
			return err
		}

		m.updatePolicies(matchingPolicy, Remove)
	}
	return nil
}

func (m *PlacementPolicyManager) updatePolicies(policy *PolicyInfo, action PodAction) {
	namespace := policy.Namespace

	if action == Remove {
		qualifyingCount := len(policy.qualifiedPods)

		if qualifyingCount == 0 {
			name := policy.Name
			m.policies.RemovePolicy(namespace, name)
			return
		}
	}

	m.policies.UpdatePolicy(namespace, policy)
}
