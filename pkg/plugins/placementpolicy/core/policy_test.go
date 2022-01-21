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

	type desiredResult struct {
		newPodManaged bool
		targetMet     bool
		valuesChanged bool
	}

	tests := []struct {
		name                 string
		action               v1alpha1.Action
		target               intstr.IntOrString
		currentQualifiedPods sets.String
		currentManagedPods   sets.String
		podToAdd             *corev1.Pod
		want                 desiredResult
	}{
		{
			name:                 "Must - new pod managed - target not met",
			action:               v1alpha1.ActionMust,
			target:               two,
			currentQualifiedPods: sets.NewString(),
			currentManagedPods:   sets.NewString(),
			podToAdd:             &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", UID: types.UID("pod1")}},
			want:                 desiredResult{newPodManaged: true, targetMet: false, valuesChanged: true},
		},
		{
			name:                 "Must - new pod managed - target met",
			action:               v1alpha1.ActionMust,
			target:               fiftyPercent,
			currentQualifiedPods: sets.NewString("pod1", "pod2", "pod3", "pod4", "pod5"),
			currentManagedPods:   sets.NewString("pod1", "pod2"),
			podToAdd:             &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod6", UID: types.UID("pod6")}},
			want:                 desiredResult{newPodManaged: true, targetMet: true, valuesChanged: true},
		},
		{
			name:                 "MustNot with no pods - new pod managed - target met",
			action:               v1alpha1.ActionMustNot,
			target:               fiftyPercent,
			currentQualifiedPods: sets.NewString(),
			currentManagedPods:   sets.NewString(),
			podToAdd:             &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", UID: types.UID("pod1")}},
			want:                 desiredResult{newPodManaged: true, targetMet: true, valuesChanged: true},
		},
		{
			name:                 "MustNot with pods - new pod managed - target met",
			action:               v1alpha1.ActionMustNot,
			target:               fiftyPercent,
			currentQualifiedPods: sets.NewString("pod1", "pod2", "pod3", "pod4", "pod5"),
			currentManagedPods:   sets.NewString("pod1", "pod2"),
			podToAdd:             &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod6", UID: types.UID("pod6")}},
			want:                 desiredResult{newPodManaged: true, targetMet: true, valuesChanged: true},
		},
		{
			name:                 "Must - new pod not managed - target met",
			action:               v1alpha1.ActionMust,
			target:               two,
			currentQualifiedPods: sets.NewString("pod1", "pod2"),
			currentManagedPods:   sets.NewString("pod1", "pod2"),
			podToAdd:             &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod3", UID: types.UID("pod3")}},
			want:                 desiredResult{newPodManaged: false, targetMet: true, valuesChanged: true},
		},
		{
			name:                 "MustNot - new pod not managed - target met",
			action:               v1alpha1.ActionMustNot,
			target:               eightyPercent,
			currentQualifiedPods: sets.NewString("pod1", "pod2", "pod3", "pod4"),
			currentManagedPods:   sets.NewString("pod1"),
			podToAdd:             &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod5", UID: types.UID("pod5")}},
			want:                 desiredResult{newPodManaged: false, targetMet: true, valuesChanged: true},
		},
		{
			name:                 "pod already included - no change",
			action:               v1alpha1.ActionMust,
			target:               two,
			currentQualifiedPods: sets.NewString("pod1", "pod2"),
			currentManagedPods:   sets.NewString("pod1", "pod2"),
			podToAdd:             &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod2", UID: types.UID("pod2")}},
			want:                 desiredResult{valuesChanged: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policyUnderTest := newPolicyInfo("ns", tt.name, tt.action, &tt.target)
			policyUnderTest.qualifiedPods = tt.currentQualifiedPods
			policyUnderTest.managedPods = tt.currentManagedPods
			policyUnderTest.setTargetMet() //since testing includes "no changes", this must be computed before addition
			copyOfOriginal := policyUnderTest

			policyUnderTest.addPodIfNotPresent(tt.podToAdd)
			resultTargetMet := policyUnderTest.targetMet

			if !tt.want.valuesChanged {
				originalQualifying := len(copyOfOriginal.qualifiedPods)
				resultQualifying := len(policyUnderTest.qualifiedPods)
				if resultQualifying != originalQualifying {
					t.Errorf("No changes expected but qualifiedPods changed. Original: %v Final: %v", originalQualifying, resultQualifying)
				}
				originalManaged := len(copyOfOriginal.managedPods)
				resultManaged := len(policyUnderTest.managedPods)
				if resultManaged != originalManaged {
					t.Errorf("No changes expected but managedPods did. Original: %v Final: %v", originalManaged, resultManaged)
				}
				if copyOfOriginal.targetMet != resultTargetMet {
					t.Errorf("No changes expected but targetMet changed. Original: %v Final: %v", copyOfOriginal.targetMet, resultTargetMet)
				}
			} else {
				if tt.want.targetMet != resultTargetMet {
					t.Errorf("targetMet mismatch. Expected: %v Actual: %v", tt.want.targetMet, resultTargetMet)
				}

				key, _ := framework.GetPodKey(tt.podToAdd)
				if tt.want.newPodManaged && !policyUnderTest.PodIsManagedByPolicy(key) {
					t.Errorf("Expected added pod to be managed by policy but it isn't.")
				}
			}

		})
	}
}

func TestRemovePodIfPresent(t *testing.T) {
	two := intstr.FromInt(2)
	three := intstr.FromInt(3)
	seventyFivePercent := intstr.FromString("75%")
	eightyPercent := intstr.FromString("80%")

	type desiredResult struct {
		qualifyingCount int
		managedCount    int
		targetMet       bool
	}

	tests := []struct {
		name                 string
		action               v1alpha1.Action
		target               intstr.IntOrString
		currentQualifiedPods sets.String
		currentManagedPods   sets.String
		podToRemove          *corev1.Pod
		want                 desiredResult
	}{
		{
			name:                 "pod not included - no change",
			action:               v1alpha1.ActionMust,
			target:               two,
			currentQualifiedPods: sets.NewString("pod1", "pod2"),
			currentManagedPods:   sets.NewString("pod1", "pod2"),
			podToRemove:          &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod5", UID: types.UID("pod5")}},
			want:                 desiredResult{qualifyingCount: 2, managedCount: 2, targetMet: true},
		},
		{
			name:                 "Must - target not met",
			action:               v1alpha1.ActionMust,
			target:               two,
			currentQualifiedPods: sets.NewString("pod1", "pod2", "pod3", "pod4"),
			currentManagedPods:   sets.NewString("pod1", "pod3"),
			podToRemove:          &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", UID: types.UID("pod1")}},
			want:                 desiredResult{qualifyingCount: 3, managedCount: 1, targetMet: false},
		},
		{
			name:                 "Must - target met",
			action:               v1alpha1.ActionMust,
			target:               seventyFivePercent,
			currentQualifiedPods: sets.NewString("pod1", "pod2", "pod3", "pod4"),
			currentManagedPods:   sets.NewString("pod1", "pod2", "pod3"),
			podToRemove:          &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod4", UID: types.UID("pod4")}},
			want:                 desiredResult{qualifyingCount: 3, managedCount: 3, targetMet: true},
		},
		{
			name:                 "MustNot - target met",
			action:               v1alpha1.ActionMustNot,
			target:               eightyPercent,
			currentQualifiedPods: sets.NewString("pod1", "pod2", "pod3", "pod4"),
			currentManagedPods:   sets.NewString("pod1", "pod3"),
			podToRemove:          &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", UID: types.UID("pod1")}},
			want:                 desiredResult{qualifyingCount: 3, managedCount: 1, targetMet: true},
		},
		{
			name:                 "MustNot - target not met",
			action:               v1alpha1.ActionMustNot,
			target:               three,
			currentQualifiedPods: sets.NewString("pod1", "pod2", "pod3", "pod4", "pod5", "pod6", "pod7", "pod8", "pod9"),
			currentManagedPods:   sets.NewString("pod1", "pod2", "pod3"),
			podToRemove:          &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod3", UID: types.UID("pod3")}},
			want:                 desiredResult{qualifyingCount: 8, managedCount: 2, targetMet: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policyUnderTest := newPolicyInfo("ns", tt.name, tt.action, &tt.target)
			policyUnderTest.qualifiedPods = tt.currentQualifiedPods
			policyUnderTest.managedPods = tt.currentManagedPods
			policyUnderTest.setTargetMet() //since testing includes "no changes", this must be computed before removal

			policyUnderTest.removePodIfPresent(tt.podToRemove)

			actualQualifyingCount := len(policyUnderTest.qualifiedPods)
			if actualQualifyingCount != tt.want.qualifyingCount {
				t.Errorf("qualifiedPods length: expected: %v actual: %v", tt.want.qualifyingCount, actualQualifyingCount)
			}

			actualManagedCount := len(policyUnderTest.managedPods)
			if actualManagedCount != tt.want.managedCount {
				t.Errorf("managedPods length: expected: %v actual: %v", tt.want.managedCount, actualManagedCount)
			}

			if policyUnderTest.targetMet != tt.want.targetMet {
				t.Errorf("targetMet: expected: %v actual: %v", tt.want.targetMet, policyUnderTest.targetMet)
			}
		})
	}
}

func TestPolicyInfoMerge(t *testing.T) {
	type policyInput struct {
		action        v1alpha1.Action
		target        intstr.IntOrString
		qualifiedPods sets.String
		managedPods   sets.String
	}

	tenPercent := intstr.FromString("10%")
	five := intstr.FromInt(5)

	tests := []struct {
		name     string
		existing policyInput
		updated  policyInput
	}{
		{
			name:     "action updated",
			existing: policyInput{action: v1alpha1.ActionMust, target: tenPercent, qualifiedPods: sets.NewString("pod1"), managedPods: sets.NewString("pod1")},
			updated:  policyInput{action: v1alpha1.ActionMustNot, target: tenPercent, qualifiedPods: sets.NewString("pod1"), managedPods: sets.NewString("pod1")},
		},
		{
			name:     "target updated",
			existing: policyInput{action: v1alpha1.ActionMust, target: five, qualifiedPods: sets.NewString("pod1"), managedPods: sets.NewString("pod1")},
			updated:  policyInput{action: v1alpha1.ActionMust, target: tenPercent, qualifiedPods: sets.NewString("pod1"), managedPods: sets.NewString("pod1")},
		},
		{
			name:     "number of pods updated",
			existing: policyInput{action: v1alpha1.ActionMust, target: five, qualifiedPods: sets.NewString("pod1"), managedPods: sets.NewString()},
			updated:  policyInput{action: v1alpha1.ActionMust, target: five, qualifiedPods: sets.NewString("pod1", "pod2"), managedPods: sets.NewString("pod2")},
		},
		{
			name:     "value of pods updated",
			existing: policyInput{action: v1alpha1.ActionMust, target: five, qualifiedPods: sets.NewString("pod5", "pod25"), managedPods: sets.NewString("pod5")},
			updated:  policyInput{action: v1alpha1.ActionMust, target: five, qualifiedPods: sets.NewString("pod1", "pod2"), managedPods: sets.NewString("pod2")},
		},
	}

	policyNamespace := "ns"
	policyName := "placement_policy"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existingPolicy := &PolicyInfo{
				Namespace:     policyNamespace,
				Name:          policyName,
				Action:        tt.existing.action,
				TargetSize:    &tt.existing.target,
				qualifiedPods: tt.existing.qualifiedPods,
				managedPods:   tt.existing.managedPods,
			}
			existingPolicy.setTargetMet()

			updatedPolicy := &PolicyInfo{
				Namespace:     policyNamespace,
				Name:          policyName,
				Action:        tt.updated.action,
				TargetSize:    &tt.updated.target,
				qualifiedPods: tt.updated.qualifiedPods,
				managedPods:   tt.updated.managedPods,
			}
			updatedPolicy.setTargetMet()

			final := updatedPolicy.merge(existingPolicy)
			final.setTargetMet()

			if final.Action != updatedPolicy.Action {
				t.Errorf("Unexpected Action value - existing: %v, updated: %v, final: %v", existingPolicy.Action, updatedPolicy.Action, final.Action)
			}

			if final.TargetSize != updatedPolicy.TargetSize {
				t.Errorf("Unexpected TargetSize value - existing: %v, updated: %v, final: %v", existingPolicy.TargetSize, updatedPolicy.TargetSize, final.TargetSize)
			}

			finalQualifying := final.qualifiedPods.List()
			for _, qp := range finalQualifying {
				if !updatedPolicy.qualifiedPods.Has(qp) {
					t.Errorf("Unexpected value in qualifiedPods - value %v exists in final and not in updated", qp)
				}
			}

			updatedQualifying := updatedPolicy.qualifiedPods.List()
			for _, up := range updatedQualifying {
				if !final.qualifiedPods.Has(up) {
					t.Errorf("Unexpected value in qualifiedPods - value %v exists in updated and not in final", up)
				}
			}

			finalManaged := final.managedPods.List()
			for _, qp := range finalManaged {
				if !updatedPolicy.managedPods.Has(qp) {
					t.Errorf("Unexpected value in managedPods - value %v exists in final and not in updated", qp)
				}
			}

			updatedManaged := updatedPolicy.managedPods.List()
			for _, up := range updatedManaged {
				if !final.managedPods.Has(up) {
					t.Errorf("Unexpected value in managedPods - value %v exists in updated and not in final", up)
				}
			}

			if final.targetMet != updatedPolicy.targetMet {
				t.Errorf("Unexpected targetMet value - existing: %v, updated: %v, final: %v", existingPolicy.targetMet, updatedPolicy.targetMet, final.targetMet)
			}
		})
	}
}
