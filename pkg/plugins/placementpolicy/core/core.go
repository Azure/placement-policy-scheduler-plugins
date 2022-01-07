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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// Manager defines the interfaces for PlacementPolicy management.
type Manager interface {
	GetPlacementPolicyForPod(context.Context, *corev1.Pod) (*v1alpha1.PlacementPolicy, error)
	RemovePodFromPolicy(*corev1.Pod)
	AddPolicy(*v1alpha1.PlacementPolicy)
	UpdatePolicy(*v1alpha1.PlacementPolicy, *v1alpha1.PlacementPolicy)
	DeletePolicy(*v1alpha1.PlacementPolicy)
	CalculateTrueTargetSize(specTarget *intstr.IntOrString, lenAllPods int, action v1alpha1.Action) (int, error)
	AddPodToPolicy(context.Context, *corev1.Pod, *v1alpha1.PlacementPolicy) (*v1alpha1.PlacementPolicy, error)
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
	policies PlacementPolicies
}

type PlacementPolicies map[string]map[string]*v1alpha1.PlacementPolicy

func NewPlacementPolicies() PlacementPolicies {
	return make(PlacementPolicies)
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
		policies:             NewPlacementPolicies(),
	}
}

func (m *PlacementPolicyManager) AddPolicy(policy *v1alpha1.PlacementPolicy) {
	policyNamespace := policy.Namespace
	policyName := policy.Name

	namespacePolicies, exists := m.policies[policyNamespace]
	if exists {
		_, nameExists := namespacePolicies[policyName]
		if nameExists {
			return
		}

		m.policies[policyNamespace][policyName] = policy
		return
	}

	var namespaceMap = make(map[string]*v1alpha1.PlacementPolicy)
	namespaceMap[policyName] = policy

	m.policies[policyNamespace] = namespaceMap
}

func (m *PlacementPolicyManager) DeletePolicy(policy *v1alpha1.PlacementPolicy) {
	policyNamespace := policy.Namespace

	namespacePolicies, exists := m.policies[policyNamespace]
	if exists {
		delete(namespacePolicies, policy.Name)
	}
}

func (m *PlacementPolicyManager) UpdatePolicy(oldPolicy *v1alpha1.PlacementPolicy, newPolicy *v1alpha1.PlacementPolicy) {
	namespace := oldPolicy.Namespace
	oldName := oldPolicy.Name

	newName := newPolicy.Name

	nameUnchanged := oldName == newName

	_, namespaceExists := m.policies[namespace]

	//todo more complex comparison logic and merging needed
	if namespaceExists {
		if nameUnchanged {
			m.policies[namespace][oldName] = newPolicy
			return
		}

		delete(m.policies[namespace], oldName)
		m.policies[namespace][newName] = newPolicy
		return
	}

	var namespaceMap = make(map[string]*v1alpha1.PlacementPolicy)
	namespaceMap[newName] = newPolicy
	m.policies[namespace] = namespaceMap
}

// GetPlacementPolicyForPod returns the placement policy for the given pod
func (m *PlacementPolicyManager) GetPlacementPolicyForPod(ctx context.Context, pod *corev1.Pod) (*v1alpha1.PlacementPolicy, error) {
	podNamespace := pod.Namespace

	namespaceList, namespaceExists := m.policies[podNamespace]

	if !namespaceExists {
		nsList, namespaceError := m.PopulateNamespacePolicies(podNamespace)
		if namespaceError != nil {
			return nil, namespaceError
		}

		for _, nsPolicy := range nsList {
			namespaceList[nsPolicy.Name] = nsPolicy
		}
	}

	// filter the placement policy list based on the pod's labels
	ppList := m.filterPlacementPolicyList(namespaceList, pod)

	if len(ppList) == 0 {
		return nil, nil
	}

	if len(ppList) > 1 {
		// if there are multiple placement policies, sort them by weight and return the first one
		sort.Sort(sort.Reverse(ByWeight(ppList)))
	}

	return ppList[0], nil
}

func (m *PlacementPolicyManager) PopulateNamespacePolicies(namespace string) (map[string]*v1alpha1.PlacementPolicy, error) {
	result := make(map[string]*v1alpha1.PlacementPolicy)

	ppList, err := m.ppLister.PlacementPolicies(namespace).List(labels.Everything())
	if err != nil {
		return result, err
	}

	for _, pp := range ppList {
		ppName := pp.Name
		result[ppName] = pp
	}

	m.policies[namespace] = result
	return result, nil
}

func (m *PlacementPolicyManager) filterPlacementPolicyList(ppList map[string]*v1alpha1.PlacementPolicy, pod *corev1.Pod) []*v1alpha1.PlacementPolicy {
	var filteredPPList []*v1alpha1.PlacementPolicy
	for _, pp := range ppList {
		labels := pp.Spec.PodSelector.MatchLabels
		if utils.HasMatchingLabels(pod.Labels, labels) {
			filteredPPList = append(filteredPPList, pp)
		}
	}
	return filteredPPList
}

func (m *PlacementPolicyManager) RemovePodFromPolicy(pod *corev1.Pod) {
	key, keyError := framework.GetPodKey(pod)
	if keyError != nil {
		return
	}

	ppList := m.policies[pod.Namespace]

	if ppList != nil {
		var ppArray []*v1alpha1.PlacementPolicy
		for _, pp := range ppList {
			ppArray = append(ppArray, pp)
		}

		associated := getAssociatedPolicy(ppArray, key)
		if associated != nil {
			policyNamespace := associated.Namespace
			policyName := associated.Name

			status := associated.Status
			act := associated.Spec.Policy.Action

			delete(status.AllQualifyingPods, key)
			podCount := len(status.AllQualifyingPods)

			delete(status.PodsManagedByPolicy, key)

			target, calcError := m.CalculateTrueTargetSize(associated.Spec.Policy.TargetSize, podCount, act)

			targetMet := status.TargetMet
			if calcError == nil {
				managedCount := len(status.PodsManagedByPolicy)
				targetMet = managedCount < target
			}

			updatedStatus := m.buildStatus(status.AllQualifyingPods, status.PodsManagedByPolicy, targetMet)
			m.updateLocalPolicy(policyNamespace, policyName, *updatedStatus)
		}
	}
}

func getAssociatedPolicy(ppList []*v1alpha1.PlacementPolicy, key string) *v1alpha1.PlacementPolicy {
	var matchingPolicy *v1alpha1.PlacementPolicy
	for _, pp := range ppList {
		ppStatus := pp.Status

		if ppStatus.AllQualifyingPods.Has(key) {
			matchingPolicy = pp
			break
		}
	}
	return matchingPolicy
}

func (m *PlacementPolicyManager) CalculateTrueTargetSize(specTarget *intstr.IntOrString, lenAllPods int, action v1alpha1.Action) (int, error) {
	target, err := intstr.GetScaledValueFromIntOrPercent(specTarget, lenAllPods, false)

	if err != nil {
		return 0, err
	}

	if action == v1alpha1.ActionMustNot {
		target = lenAllPods - target
	}

	return target, nil
}

func (m *PlacementPolicyManager) AddPodToPolicy(ctx context.Context, pod *corev1.Pod, pp *v1alpha1.PlacementPolicy) (*v1alpha1.PlacementPolicy, error) {
	podKey, err := framework.GetPodKey(pod)
	if err != nil {
		return pp, err
	}

	policyNamespace := pp.Namespace
	policyName := pp.Name

	specTarget := pp.Spec.Policy.TargetSize

	ppState := pp.Status

	allPods := ppState.AllQualifyingPods.Insert(podKey) // add pod id to "all pods"
	lenAllPods := len(allPods)                          //new "all pods"

	calcTarget, targetErr := m.CalculateTrueTargetSize(specTarget, lenAllPods, pp.Spec.Policy.Action)
	if targetErr != nil {
		return pp, targetErr
	}

	managedPods := ppState.PodsManagedByPolicy
	lenManaged := len(managedPods)

	if lenManaged >= calcTarget {
		//after re-calculating target, the number of pods currently managed by the policy meets or exceeds the calculated target
		metState := m.buildStatus(allPods, managedPods, true)
		updatedPolicy := m.updateLocalPolicy(policyNamespace, policyName, *metState)
		return updatedPolicy, nil
	}

	managedPods = managedPods.Insert(podKey)
	lenManaged = lenManaged + 1

	updatedState := m.buildStatus(allPods, managedPods, (lenManaged >= calcTarget))
	policy := m.updateLocalPolicy(policyNamespace, policyName, *updatedState)
	return policy, nil
}

func (m *PlacementPolicyManager) updateLocalPolicy(namespace string, name string, updatedStatus v1alpha1.PlacementPolicyStatus) *v1alpha1.PlacementPolicy {
	m.policies[namespace][name].Status = updatedStatus
	return m.policies[namespace][name]
}

func (m *PlacementPolicyManager) buildStatus(allPods sets.String, managedPods sets.String, targetMet bool) *v1alpha1.PlacementPolicyStatus {
	updatedState := new(v1alpha1.PlacementPolicyStatus)
	updatedState.AllQualifyingPods = allPods
	updatedState.PodsManagedByPolicy = managedPods
	updatedState.TargetMet = targetMet
	return updatedState
}
