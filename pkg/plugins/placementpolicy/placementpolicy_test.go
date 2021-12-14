package placementpolicy

import (
	"reflect"
	"testing"

	"github.com/Azure/placement-policy-scheduler-plugins/apis/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

func TestGroupNodesWithLabels(t *testing.T) {
	tests := []struct {
		name     string
		nodeList []*corev1.Node
		labels   map[string]string
		want     func() map[string]*framework.NodeInfo
	}{
		{
			name:     "no nodes",
			nodeList: []*corev1.Node{},
			labels:   map[string]string{"foo": "bar"},
			want:     func() map[string]*framework.NodeInfo { return map[string]*framework.NodeInfo{} },
		},
		{
			name: "no matching nodes",
			nodeList: []*corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "node1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "node2"}},
			},
			labels: map[string]string{"foo": "bar"},
			want:   func() map[string]*framework.NodeInfo { return map[string]*framework.NodeInfo{} },
		},
		{
			name: "matching nodes found",
			nodeList: []*corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"foo": "bar"}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "node2"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "node3", Labels: map[string]string{"foo": "bar", "baz": "qux"}}},
			},
			labels: map[string]string{"foo": "bar"},
			want: func() map[string]*framework.NodeInfo {
				n1 := framework.NewNodeInfo()
				n1.SetNode(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"foo": "bar"}}})

				n3 := framework.NewNodeInfo()
				n3.SetNode(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node3", Labels: map[string]string{"foo": "bar", "baz": "qux"}}})

				return map[string]*framework.NodeInfo{
					"node1": n1,
					"node3": n3,
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := groupNodesWithLabels(tt.nodeList, tt.labels)
			if len(got) != len(tt.want()) {
				t.Errorf("groupNodesWithLabels(%v, %v) = %v, want %v", tt.nodeList, tt.labels, got, tt.want())
			}
			for k := range tt.want() {
				if _, ok := got[k]; !ok {
					t.Errorf("groupNodesWithLabels(%v, %v) = %v, want %v", tt.nodeList, tt.labels, got, tt.want())
				}
			}
		})
	}
}

func TestGroupPodsBasedOnNodePreference(t *testing.T) {
	tests := []struct {
		name                   string
		podList                []*corev1.Pod
		pod                    *corev1.Pod
		nodeWithMatchingLabels map[string]*corev1.Node
		want                   []*corev1.Pod
	}{
		{
			name:                   "no pods",
			podList:                []*corev1.Pod{},
			pod:                    &corev1.Pod{},
			nodeWithMatchingLabels: map[string]*corev1.Node{},
			want:                   []*corev1.Pod{},
		},
		{
			name: "skip current pod",
			podList: []*corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "pod1", UID: types.UID("pod1")}},
			},
			pod:                    &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", UID: types.UID("pod1")}},
			nodeWithMatchingLabels: map[string]*corev1.Node{},
			want:                   []*corev1.Pod{},
		},
		{
			name: "pod with node name exists",
			podList: []*corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "pod1", UID: types.UID("pod1")}},
				{ObjectMeta: metav1.ObjectMeta{Name: "pod2", UID: types.UID("pod2")}, Spec: corev1.PodSpec{NodeName: "node1"}},
			},
			pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", UID: types.UID("pod1")}},
			nodeWithMatchingLabels: map[string]*corev1.Node{
				"node1": {ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"foo": "bar"}}},
			},
			want: []*corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "pod2", UID: types.UID("pod2")}, Spec: corev1.PodSpec{NodeName: "node1"}},
			},
		},
		{
			name: "no node name or annotation",
			podList: []*corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "pod1", UID: types.UID("pod1")}},
				{ObjectMeta: metav1.ObjectMeta{Name: "pod2", UID: types.UID("pod2")}},
			},
			pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", UID: types.UID("pod1")}},
			nodeWithMatchingLabels: map[string]*corev1.Node{
				"node1": {ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"foo": "bar"}}},
			},
			want: []*corev1.Pod{},
		},
		{
			name: "no node name but annotation exists",
			podList: []*corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "pod1", UID: types.UID("pod1")}},
				{ObjectMeta: metav1.ObjectMeta{Name: "pod2", UID: types.UID("pod2"), Annotations: map[string]string{v1alpha1.PlacementPolicyPreferenceAnnotationKey: "true"}}},
			},
			pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", UID: types.UID("pod1")}},
			nodeWithMatchingLabels: map[string]*corev1.Node{
				"node1": {ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"foo": "bar"}}},
			},
			want: []*corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "pod2", UID: types.UID("pod2"), Annotations: map[string]string{v1alpha1.PlacementPolicyPreferenceAnnotationKey: "true"}}},
			},
		},
		{
			name: "annotation exists but no matching node",
			podList: []*corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "pod1", UID: types.UID("pod1")}},
				{ObjectMeta: metav1.ObjectMeta{Name: "pod2", UID: types.UID("pod2"), Annotations: map[string]string{v1alpha1.PlacementPolicyPreferenceAnnotationKey: "false"}}},
			},
			pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", UID: types.UID("pod1")}},
			nodeWithMatchingLabels: map[string]*corev1.Node{
				"node1": {ObjectMeta: metav1.ObjectMeta{Name: "node1", Labels: map[string]string{"foo": "bar"}}},
			},
			want: []*corev1.Pod{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := groupPodsBasedOnNodePreference(tt.podList, tt.pod, tt.nodeWithMatchingLabels)
			if len(got) != len(tt.want) {
				t.Errorf("groupPodsBasedOnNodePreference(%v, %v, %v) = %v, want %v", tt.podList, tt.pod, tt.nodeWithMatchingLabels, got, tt.want)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("groupPodsBasedOnNodePreference(%v, %v, %v) = %v, want %v", tt.podList, tt.pod, tt.nodeWithMatchingLabels, got, tt.want)
			}
		})
	}
}
