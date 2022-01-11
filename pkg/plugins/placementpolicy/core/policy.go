package core

import (
	"github.com/Azure/placement-policy-scheduler-plugins/apis/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

type PolicyInfo struct {
	Namespace           string
	Name                string
	Action              v1alpha1.Action
	TargetSize          *intstr.IntOrString
	allQualifyingPods   sets.String
	podsManagedByPolicy sets.String
	targetMet           bool
}

func newPolicyInfo(namespace string, name string, action v1alpha1.Action, targetSize *intstr.IntOrString) *PolicyInfo {
	policy := &PolicyInfo{
		Namespace:           namespace,
		Name:                name,
		Action:              action,
		TargetSize:          targetSize,
		allQualifyingPods:   sets.NewString(),
		podsManagedByPolicy: sets.NewString(),
		targetMet:           false,
	}
	return policy
}

type PolicyInfos map[string]map[string]*PolicyInfo

func NewPolicyInfos() PolicyInfos {
	return make(PolicyInfos)
}

func (p *PolicyInfo) merge(existing *PolicyInfo) *PolicyInfo {
	existing.targetMet = p.targetMet
	existing.allQualifyingPods = sets.NewString()
	existing.podsManagedByPolicy = sets.NewString()

	if len(p.allQualifyingPods) > 0 {
		pods := p.allQualifyingPods.List()
		for _, pod := range pods {
			existing.allQualifyingPods.Insert(pod)
		}
	}

	if len(p.podsManagedByPolicy) > 0 {
		pods := p.podsManagedByPolicy.List()
		for _, pod := range pods {
			existing.podsManagedByPolicy.Insert(pod)
		}
	}

	return existing
}

func (p *PolicyInfo) removePodIfPresent(pod *corev1.Pod) error {
	key, keyError := framework.GetPodKey(pod)
	if keyError != nil {
		return keyError
	}

	if !p.PodQualifiesForPolicy(key) {
		return nil
	}

	allQualifying := p.allQualifyingPods
	p.allQualifyingPods = allQualifying.Delete(key)

	if p.PodIsManagedByPolicy(key) {
		managed := p.podsManagedByPolicy
		p.podsManagedByPolicy = managed.Delete(key)
	}

	err := p.setTargetMet()
	return err
}

func (p *PolicyInfo) addPodIfNotPresent(pod *corev1.Pod) error {
	key, keyError := framework.GetPodKey(pod)
	if keyError != nil {
		return keyError
	}

	if p.PodQualifiesForPolicy(key) {
		return nil
	}

	qualifyingPods := p.allQualifyingPods
	p.allQualifyingPods = qualifyingPods.Insert(key)

	targetMetWithoutNewPod, targetErr := p.computeTargetMet()
	if targetErr != nil {
		return targetErr
	}

	if targetMetWithoutNewPod {
		p.targetMet = true
		return nil
	}

	managedPods := p.podsManagedByPolicy
	p.podsManagedByPolicy = managedPods.Insert(key)

	setErr := p.setTargetMet()
	return setErr
}

func (p *PolicyInfo) calculateTrueTargetSize() (int, error) {
	specTarget := p.TargetSize
	lenAllPods := len(p.allQualifyingPods)

	target, err := intstr.GetScaledValueFromIntOrPercent(specTarget, lenAllPods, false)

	if err != nil {
		return 0, err
	}

	if p.Action == v1alpha1.ActionMustNot {
		target = lenAllPods - target
	}

	return target, nil
}

func (p *PolicyInfo) setTargetMet() error {
	targetMet, err := p.computeTargetMet()
	if err != nil {
		return err
	}
	p.targetMet = targetMet
	return nil
}

func (p *PolicyInfo) computeTargetMet() (bool, error) {
	target, calcError := p.calculateTrueTargetSize()
	if calcError != nil {
		return false, calcError
	}

	managedCount := len(p.podsManagedByPolicy)
	targetMet := managedCount < target
	return targetMet, nil
}

func (p *PolicyInfo) PodQualifiesForPolicy(podKey string) bool {
	return p.allQualifyingPods.Has(podKey)
}

func (p *PolicyInfo) PodIsManagedByPolicy(podKey string) bool {
	return p.podsManagedByPolicy.Has(podKey)
}
