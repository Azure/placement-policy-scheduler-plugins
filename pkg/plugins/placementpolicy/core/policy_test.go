package core

import (
	"testing"

	"github.com/Azure/placement-policy-scheduler-plugins/apis/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

func TestAddPodIfNotPresent(t *testing.T) {
	two := intstr.FromInt(2)
	fiftyPercent := intstr.FromString("50%")
	eightyPercent := intstr.FromString("80%")

	tests := []struct {
		name                 string
		action               v1alpha1.Action
		target               intstr.IntOrString
		currentQualifiedPods sets.String
		currentManagedPods   sets.String
		podToAdd             *corev1.Pod
		wantPolicyManaged    bool
	}{
		{
			name:                 "target threshold not met - Must and no pods",
			action:               v1alpha1.ActionMust,
			target:               two,
			currentQualifiedPods: sets.NewString(),
			currentManagedPods:   sets.NewString(),
			podToAdd:             &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", UID: types.UID("pod1")}},
			wantPolicyManaged:    true,
		},
		{
			name:                 "target threshold not met - Must with pods",
			action:               v1alpha1.ActionMust,
			target:               fiftyPercent,
			currentQualifiedPods: sets.NewString("pod1", "pod2", "pod3", "pod4", "pod5"),
			currentManagedPods:   sets.NewString("pod1", "pod2"),
			podToAdd:             &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod6", UID: types.UID("pod6")}},
			wantPolicyManaged:    true,
		},
		{
			name:                 "target threshold not met - MustNot and no pods",
			action:               v1alpha1.ActionMustNot,
			target:               fiftyPercent,
			currentQualifiedPods: sets.NewString(),
			currentManagedPods:   sets.NewString(),
			podToAdd:             &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", UID: types.UID("pod1")}},
			wantPolicyManaged:    true,
		},
		{
			name:                 "target threshold not met - MustNot with pods",
			action:               v1alpha1.ActionMustNot,
			target:               fiftyPercent,
			currentQualifiedPods: sets.NewString("pod1", "pod2", "pod3", "pod4", "pod5"),
			currentManagedPods:   sets.NewString("pod1", "pod2"),
			podToAdd:             &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod6", UID: types.UID("pod6")}},
			wantPolicyManaged:    true,
		},
		{
			name:                 "target threshold met - Must",
			action:               v1alpha1.ActionMust,
			target:               two,
			currentQualifiedPods: sets.NewString("pod1", "pod2"),
			currentManagedPods:   sets.NewString("pod1", "pod2"),
			podToAdd:             &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod3", UID: types.UID("pod3")}},
			wantPolicyManaged:    false,
		},
		{
			name:                 "target threshold met - MustNot",
			action:               v1alpha1.ActionMustNot,
			target:               eightyPercent,
			currentQualifiedPods: sets.NewString("pod1", "pod2", "pod3", "pod4"),
			currentManagedPods:   sets.NewString("pod1"),
			podToAdd:             &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod5", UID: types.UID("pod5")}},
			wantPolicyManaged:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policyUnderTest := newPolicyInfo("ns", tt.name, tt.action, &tt.target)
			policyUnderTest.allQualifyingPods = tt.currentQualifiedPods
			policyUnderTest.podsManagedByPolicy = tt.currentManagedPods

			policyUnderTest.addPodIfNotPresent(tt.podToAdd)
			key, _ := framework.GetPodKey(tt.podToAdd)

			podManaged := policyUnderTest.PodIsManagedByPolicy(key)

			if podManaged != tt.wantPolicyManaged {
				t.Errorf("policy %v, addPodIfNotPresent(%v), PodIsManagedByPolicy %v, wantPolicyManaged %v", policyUnderTest, tt.podToAdd, podManaged, tt.wantPolicyManaged)
			}
		})
	}
}
