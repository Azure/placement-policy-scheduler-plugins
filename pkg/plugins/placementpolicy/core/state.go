package core

import "k8s.io/apimachinery/pkg/types"

type PlacementPolicyPodInfos map[types.UID]*PlacementPolicyPodInfo

type PlacementPolicyPodInfo struct {
	PodName                         string
	PreferredNodeWithMatchingLabels bool
	PodAnnotated                    bool
}

func NewPlacementPolicyPodInfos() PlacementPolicyPodInfos {
	return make(PlacementPolicyPodInfos)
}

func (p PlacementPolicyPodInfos) Get(podUID types.UID) *PlacementPolicyPodInfo {
	return p[podUID]
}

func (p PlacementPolicyPodInfos) Set(podUID types.UID, podInfo *PlacementPolicyPodInfo) {
	p[podUID] = podInfo
}

func (p PlacementPolicyPodInfos) Delete(podUID types.UID) {
	delete(p, podUID)
}

func (p PlacementPolicyPodInfos) Clone() PlacementPolicyPodInfos {
	podInfos := make(PlacementPolicyPodInfos)
	for key, podInfo := range p {
		podInfos.Set(key, podInfo)
	}
	return podInfos
}
