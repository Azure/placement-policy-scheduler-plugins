package core

import "github.com/Azure/placement-policy-scheduler-plugins/apis/v1alpha1"

type ByWeight []*v1alpha1.PlacementPolicy

func (a ByWeight) Len() int { return len(a) }

func (a ByWeight) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a ByWeight) Less(i, j int) bool {
	return a[i].Spec.Weight > a[j].Spec.Weight
}
