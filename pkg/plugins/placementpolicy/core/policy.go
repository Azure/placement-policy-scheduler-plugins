package core

import (
	"github.com/Azure/placement-policy-scheduler-plugins/apis/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

type PolicyInfo struct {
	Namespace     string
	Name          string
	Action        v1alpha1.Action
	TargetSize    *intstr.IntOrString
	matchedPods   sets.String
	qualifiedPods sets.String
	managedPods   sets.String
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

type PolicyInfos map[string]map[string]*PolicyInfo

func NewPolicyInfos() PolicyInfos {
	return make(PolicyInfos)
}

func (p *PolicyInfo) merge(existing *PolicyInfo) *PolicyInfo {
	existing.Action = p.Action
	existing.TargetSize = p.TargetSize
	existing.targetMet = p.targetMet

	tempMatched := sets.NewString()
	if len(p.matchedPods) > 0 {
		pods := p.matchedPods.List()
		for _, pod := range pods {
			tempMatched = tempMatched.Insert(pod)
		}
	}
	existing.matchedPods = tempMatched

	tempQualified := sets.NewString()
	if len(p.qualifiedPods) > 0 {
		pods := p.qualifiedPods.List()
		for _, pod := range pods {
			tempQualified = tempQualified.Insert(pod)
		}
	}
	existing.qualifiedPods = tempQualified

	tempManaged := sets.NewString()
	if len(p.managedPods) > 0 {
		pods := p.managedPods.List()
		for _, pod := range pods {
			tempManaged = tempManaged.Insert(pod)
		}
	}
	existing.managedPods = tempManaged

	return existing
}

func (p *PolicyInfo) addMatch(pod *corev1.Pod) error {
	key, err := framework.GetPodKey(pod)
	if keyError != nil {
		return keyError
	}

	p.matchedPods = p.matchedPods.Insert(key)
	return nil
}

func (p *PolicyInfo) removePodIfPresent(pod *corev1.Pod) error {
	key, keyError := framework.GetPodKey(pod)
	if keyError != nil {
		return keyError
	}

	if !p.PodQualifiesForPolicy(key) {
		return nil
	}

	p.qualifiedPods = p.qualifiedPods.Delete(key)

	if p.PodIsManagedByPolicy(key) {
		p.managedPods = p.managedPods.Delete(key)
	}

	err := p.setTargetMet()
	return err
}

func (p *PolicyInfo) addPodIfNotPresent(pod *corev1.Pod) error {
	key, keyError := framework.GetPodKey(pod)
	if keyError != nil {
		return keyError
	}

	//if pod is already in the list, do nothing
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
	err := p.setTargetMet()
	return err
}

func (p *PolicyInfo) setTargetMet() error {
	specTarget := p.TargetSize
	lenAllPods := len(p.qualifiedPods)

	target, calcError := intstr.GetScaledValueFromIntOrPercent(specTarget, lenAllPods, false)

	if calcError != nil {
		return calcError
	}

	if p.Action == v1alpha1.ActionMustNot {
		target = lenAllPods - target
	}

	managedCount := len(p.managedPods)
	p.targetMet = managedCount >= target //since the TargetSize is rounded down, the expectation that it will only meet/equal and never exceed
	return nil
}

func (p *PolicyInfo) PodMatchesPolicy(podKey string) bool {
	return p.matchedPods.Has(podKey)
}

func (p *PolicyInfo) PodQualifiesForPolicy(podKey string) bool {
	return p.qualifiedPods.Has(podKey)
}

func (p *PolicyInfo) PodIsManagedByPolicy(podKey string) bool {
	return p.managedPods.Has(podKey)
}
