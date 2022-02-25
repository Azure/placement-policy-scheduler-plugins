package core

import (
	"sync"

	"github.com/Azure/placement-policy-scheduler-plugins/apis/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

type PolicyInfo struct {
	// PlacementPolicy namespace - from CRD
	Namespace     string
	// PlacementPolicy name - from CRD
	Name          string
	//policy's action - from CRD
	Action        v1alpha1.Action
	//policy's target - from CRD
	TargetSize    *intstr.IntOrString
	//collection of pods matched if the policy is `BestEffort`; **not** used for computation
	matchedPods   sets.String
	//collection of pods that could be managed by the policy; used for computation as the **total**
	qualifiedPods sets.String
	//collection of pods assigned to a node according to the policy; used for computation
	managedPods   sets.String
	//does the ratio of managed-to-qualified meet the `TargetSize`
	targetMet     bool
}

func newPolicyInfo(namespace string, name string, action v1alpha1.Action, targetSize *intstr.IntOrString) *PolicyInfo {
	policy := &PolicyInfo{
		Namespace:     namespace,
		Name:          name,
		Action:        action,
		TargetSize:    targetSize,
		matchedPods:   sets.NewString(),
		qualifiedPods: sets.NewString(),
		managedPods:   sets.NewString(),
		targetMet:     false,
	}
	return policy
}

type PolicyInfos struct {
	sync.RWMutex
	//map (by `Namespace`) of map (by `Name`) of `PolicyInfo`
	internal map[string]map[string]*PolicyInfo
}

func NewPolicyInfos() PolicyInfos {
	return PolicyInfos{
		internal: make(map[string]map[string]*PolicyInfo),
	}
}

// GetPolicy gets the PolicyInfo if it exists, otherwise it creates and returns the newly created PolicyInfo
func (pi *PolicyInfos) GetPolicy(policyNamespace string, policyName string, action v1alpha1.Action, targetSize *intstr.IntOrString) *PolicyInfo {
	var policy *PolicyInfo = nil
	var policyExists bool = false

	pi.RLock()
	namespacePolicies, namespaceExists := pi.internal[policyNamespace]
	if namespaceExists {
		policy, policyExists = namespacePolicies[policyName]
	}
	pi.RUnlock()

	if policyExists {
		return policy
	}

	pi.Lock()
	if !namespaceExists {
		pi.internal[policyNamespace] = make(map[string]*PolicyInfo)
	}

	created := newPolicyInfo(policyNamespace, policyName, action, targetSize)
	pi.internal[policyNamespace][policyName] = created

	pi.Unlock()

	return created
}

// RemovePolicy removes the policy from the in-memory collection
func (pi *PolicyInfos) RemovePolicy(policyNamespace string, policyName string) {
	pi.Lock()
	delete(pi.internal[policyNamespace], policyName)
	pi.Unlock()
}

// GetPoliciesByNamespace returns all policies in the namespace
func (pi *PolicyInfos) GetPoliciesByNamespace(policyNamespace string) map[string]*PolicyInfo {
	pi.RLock()
	namespacePolicies := pi.internal[policyNamespace]
	pi.RUnlock()
	return namespacePolicies
}

// UpdatePolicy updates the policy held in-memory
func (pi *PolicyInfos) UpdatePolicy(policyNamespace string, policy *PolicyInfo) {
	policyName := policy.Name
	pi.Lock()
	existing := pi.internal[policyNamespace][policyName]
	pi.internal[policyNamespace][policyName] = policy.merge(existing)
	pi.Unlock()
}

func (p *PolicyInfo) merge(existing *PolicyInfo) *PolicyInfo {
	existing.Action = p.Action
	existing.TargetSize = p.TargetSize
	existing.targetMet = p.targetMet

	tempMatched := sets.NewString()
	if len(p.matchedPods) > 0 {
		tempMatched = tempMatched.Insert(p.matchedPods.List()...)
	}
	existing.matchedPods = tempMatched

	tempQualified := sets.NewString()
	if len(p.qualifiedPods) > 0 {
		tempQualified = tempQualified.Insert(p.qualifiedPods.List()...)
	}
	existing.qualifiedPods = tempQualified

	tempManaged := sets.NewString()
	if len(p.managedPods) > 0 {
		tempManaged = tempManaged.Insert(p.managedPods.List()...)
	}
	existing.managedPods = tempManaged

	return existing
}

// addMatch adds the pod to the policy's matchedPods collection
func (p *PolicyInfo) addMatch(pod *corev1.Pod) error {
	key, err := framework.GetPodKey(pod)
	if err != nil {
		return err
	}

	p.matchedPods = p.matchedPods.Insert(key)
	return nil
}

// removePodIfPresent removes the pod from the policy's qualifiedPods and managedPods collections as appropriate
func (p *PolicyInfo) removePodIfPresent(pod *corev1.Pod) error {
	key, err := framework.GetPodKey(pod)
	if err != nil {
		return err
	}

	if !p.PodQualifiesForPolicy(key) {
		return nil
	}

	p.qualifiedPods = p.qualifiedPods.Delete(key)

	if p.PodIsManagedByPolicy(key) {
		p.managedPods = p.managedPods.Delete(key)
	}

	return p.setTargetMet()
}

// addPodIfNotPresent adds the pod to the policy's qualifiedPods and managedPods collections as appropriate
func (p *PolicyInfo) addPodIfNotPresent(pod *corev1.Pod) error {
	key, keyError := framework.GetPodKey(pod)
	if keyError != nil {
		return keyError
	}

	// if pod is already in the list, do nothing
	if p.PodQualifiesForPolicy(key) {
		return nil
	}

	//if policy is `BestEffort`, this will be true
	if p.PodMatchesPolicy(key) {
		p.matchedPods = p.matchedPods.Delete(key) //once added, don't need to worry about matched anymore
	}

	p.qualifiedPods = p.qualifiedPods.Insert(key)

	targetError := p.setTargetMet()
	if targetError != nil {
		return targetError
	}

	//if target met, pod doesn't need to be managed
	if p.targetMet {
		return nil
	}

	p.managedPods = p.managedPods.Insert(key)
	return p.setTargetMet()
}

func (p *PolicyInfo) setTargetMet() error {
	specTarget := p.TargetSize
	lenAllPods := len(p.qualifiedPods)

	target, err := intstr.GetScaledValueFromIntOrPercent(specTarget, lenAllPods, false)

	if err != nil {
		return err
	}

	if p.Action == v1alpha1.ActionMustNot {
		target = lenAllPods - target
	}

	managedCount := len(p.managedPods)
	p.targetMet = managedCount >= target //since the TargetSize is rounded down, the expectation that it will only meet/equal and never exceed
	return nil
}

// PodMatchesPolicy returns whether or not the pod key is matched to the BestEffort policy during Filter
func (p *PolicyInfo) PodMatchesPolicy(podKey string) bool {
	return p.matchedPods.Has(podKey)
}

// PodQualifiesForPolicy returns whether or not the pod key is in the list of qualifying pods
func (p *PolicyInfo) PodQualifiesForPolicy(podKey string) bool {
	return p.qualifiedPods.Has(podKey)
}

// PodIsManagedByPolicy returns whether or not the pod key is in the list of pods assigned to node(s) in accordance with the policy
func (p *PolicyInfo) PodIsManagedByPolicy(podKey string) bool {
	return p.managedPods.Has(podKey)
}
