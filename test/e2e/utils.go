//go:build e2e
// +build e2e

package e2e

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

func NewDeployment(namespace string, name string, replicas int32, labels map[string]string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "test-deployment",
							Image:           e2epod.GetDefaultTestImage(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"/bin/sleep", "10000"},
						},
					},
				},
			},
		},
	}
}

func NewStatefulSet(namespace string, name string, replicas int32, labels map[string]string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "test-statefulset",
							Image:           e2epod.GetDefaultTestImage(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"/bin/sleep", "10000"},
						},
					},
				},
			},
		},
	}
}

func NewDaemonSet(namespace string, name string, labels map[string]string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "test-daemonset",
							Image:           e2epod.GetDefaultTestImage(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"/bin/sleep", "10000"},
						},
					},
				},
			},
		},
	}
}
