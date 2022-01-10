//go:build e2e
// +build e2e

package e2e

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

const (
	schedulerName = "placement-policy-plugins-scheduler"
)

func newDeployment(namespace, name string, replicas int32, labels map[string]string) *appsv1.Deployment {
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
					SchedulerName: schedulerName,
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

func newNodeResourceDeployment(namespace, name string, replicas int32, labels map[string]string) *appsv1.Deployment {
	resList := map[v1.ResourceName]string{
		v1.ResourceCPU:        "1",
		v1.ResourceMemory:     "0",
		"example.com/deviceA": "1",
		"example.com/deviceB": "1",
	}
	res := v1.ResourceList{}
	for k, v := range resList {
		res[k] = resource.MustParse(v)
	}
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
					SchedulerName: schedulerName,
					Containers: []corev1.Container{
						{
							Name:            "test-deployment",
							Image:           e2epod.GetDefaultTestImage(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"/bin/sleep", "10000"},
							Resources: corev1.ResourceRequirements{
								Limits:   res,
								Requests: res,
							},
						},
					},
				},
			},
		},
	}
}

func newStatefulSet(namespace, name string, replicas int32, labels map[string]string) *appsv1.StatefulSet {
	resList := map[v1.ResourceName]string{
		v1.ResourceCPU:    "3000m",
		v1.ResourceMemory: "300Mi",
	}
	res := v1.ResourceList{}
	for k, v := range resList {
		res[k] = resource.MustParse(v)
	}

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					SchedulerName: schedulerName,
					Containers: []corev1.Container{
						{
							Name:            "test-statefulset",
							Image:           e2epod.GetDefaultTestImage(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"/bin/sleep", "10000"},
							Resources: corev1.ResourceRequirements{
								Limits:   res,
								Requests: res,
							},
						},
					},
				},
			},
		},
	}
}
