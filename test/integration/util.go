package integration

import (
	"context"

	"github.com/Azure/placement-policy-scheduler-plugins/apis/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/kube-scheduler/config/v1beta2"
	"k8s.io/kubernetes/pkg/scheduler/apis/config"
	schdscheme "k8s.io/kubernetes/pkg/scheduler/apis/config/scheme"
)

var (
	NodeSelectorLabels = map[string]string{"node": "want"}
	PodSelectorLabels  = map[string]string{"app": "nginx"}
)

// PodScheduled returns true if a node is assigned to the given pod.
func PodScheduled(c kubernetes.Interface, podNamespace, podName string) bool {
	pod, err := c.CoreV1().Pods(podNamespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		// This could be a connection error so we want to retry.
		klog.ErrorS(err, "Failed to get pod", "pod", klog.KRef(podNamespace, podName))
		return false
	}
	return pod.Spec.NodeName != ""
}

// MakePlacementPolicy
func MakePlacementPolicy(mode v1alpha1.EnforcementMode, targetSize intstr.IntOrString, action v1alpha1.Action, name, namespace string) *v1alpha1.PlacementPolicy {

	return &v1alpha1.PlacementPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1alpha1.PlacementPolicySpec{
			Weight:          100,
			EnforcementMode: mode,
			PodSelector: &metav1.LabelSelector{
				MatchLabels: PodSelectorLabels,
			},
			NodeSelector: &metav1.LabelSelector{
				MatchLabels: NodeSelectorLabels,
			},
			Policy: &v1alpha1.Policy{Action: action, TargetSize: &targetSize},
		},
		Status: v1alpha1.PlacementPolicyStatus{},
	}
}

// https://github.com/kubernetes-sigs/scheduler-plugins/blob/478a9cb0867c10821bfac3bf1a2be3434796af81/test/util/framework.go
// NewDefaultSchedulerComponentConfig returns a default scheduler cc object.
// We need this function due to k/k#102796 - default profile needs to built manually.
func NewDefaultSchedulerComponentConfig() (config.KubeSchedulerConfiguration, error) {
	var versionedCfg v1beta2.KubeSchedulerConfiguration
	schdscheme.Scheme.Default(&versionedCfg)
	cfg := config.KubeSchedulerConfiguration{}
	if err := schdscheme.Scheme.Convert(&versionedCfg, &cfg, nil); err != nil {
		return config.KubeSchedulerConfiguration{}, err
	}
	return cfg, nil
}
