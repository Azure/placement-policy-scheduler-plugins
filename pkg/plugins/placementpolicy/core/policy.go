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
	existing.Action = p.Action
	existing.TargetSize = p.TargetSize
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

	p.allQualifyingPods = p.allQualifyingPods.Delete(key)

	if p.PodIsManagedByPolicy(key) {
		p.podsManagedByPolicy = p.podsManagedByPolicy.Delete(key)
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

	p.allQualifyingPods = p.allQualifyingPods.Insert(key)

	targetErr := p.setTargetMet()
	if targetErr != nil {
		return targetErr
	}

	//if target was met without also adding the pod to the "managed" list, then nothing else to do
	if p.targetMet {
		return nil
	}

	p.podsManagedByPolicy = p.podsManagedByPolicy.Insert(key)

	err := p.setTargetMet()
	return err
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
	target, calcError := p.calculateTrueTargetSize()
	if calcError != nil {
		return calcError
	}

	managedCount := len(p.podsManagedByPolicy)
	p.targetMet = managedCount >= target //since the TargetSize is rounded down, the expectation that it will only meet/equal and never exceed
	return nil
}

func (p *PolicyInfo) PodQualifiesForPolicy(podKey string) bool {
	return p.allQualifyingPods.Has(podKey)
}

func (p *PolicyInfo) PodIsManagedByPolicy(podKey string) bool {
	return p.podsManagedByPolicy.Has(podKey)
}
