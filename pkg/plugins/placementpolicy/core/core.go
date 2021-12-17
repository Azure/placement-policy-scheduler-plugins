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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// Manager defines the interfaces for PlacementPolicy management.
type Manager interface {
	GetPlacementPolicyForPod(context.Context, *corev1.Pod) (*v1alpha1.PlacementPolicy, error)
	GetPodsWithLabels(context.Context, map[string]string) ([]*corev1.Pod, error)
	AnnotatePod(context.Context, *corev1.Pod, *v1alpha1.PlacementPolicy, bool) (*corev1.Pod, error)
	GetPlacementPolicy(context.Context, string, string) (*v1alpha1.PlacementPolicy, error)
}

type PlacementPolicyManager struct {
	// client is a clientset for the kube API server.
	client kubernetes.Interface
	// client is a placementPolicy client
	ppClient ppclientset.Interface
	// podLister is pod lister
	podLister corelisters.PodLister
	// snapshotSharedLister is pod shared list
	snapshotSharedLister framework.SharedLister
	// ppLister is placementPolicy lister
	ppLister pplisters.PlacementPolicyLister
}

func NewPlacementPolicyManager(
	client kubernetes.Interface,
	ppClient ppclientset.Interface,
	snapshotSharedLister framework.SharedLister,
	ppInformer ppinformers.PlacementPolicyInformer,
	podLister corelisters.PodLister) *PlacementPolicyManager {
	return &PlacementPolicyManager{
		client:               client,
		ppClient:             ppClient,
		snapshotSharedLister: snapshotSharedLister,
		ppLister:             ppInformer.Lister(),
		podLister:            podLister,
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

func (m *PlacementPolicyManager) GetPodsWithLabels(ctx context.Context, podLabels map[string]string) ([]*corev1.Pod, error) {
	return m.podLister.List(labels.Set(podLabels).AsSelector())
}

// AnnotatePod annotates the pod with the placement policy.
func (m *PlacementPolicyManager) AnnotatePod(ctx context.Context, pod *corev1.Pod, pp *v1alpha1.PlacementPolicy, preferredNodeWithMatchingLabels bool) (*corev1.Pod, error) {
	annotations := map[string]string{}
	if pod.Annotations != nil {
		annotations = pod.Annotations
	}

	preference := "false"
	if preferredNodeWithMatchingLabels {
		preference = "true"
	}
	annotations[v1alpha1.PlacementPolicyAnnotationKey] = pp.Name
	annotations[v1alpha1.PlacementPolicyPreferenceAnnotationKey] = preference
	pod.Annotations = annotations
	return m.client.CoreV1().Pods(pod.Namespace).Update(ctx, pod, metav1.UpdateOptions{})
}

func (m *PlacementPolicyManager) GetPlacementPolicy(ctx context.Context, namespace, name string) (*v1alpha1.PlacementPolicy, error) {
	return m.ppLister.PlacementPolicies(namespace).Get(name)
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
