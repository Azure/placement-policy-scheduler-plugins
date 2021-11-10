//go:build e2e
// +build e2e

package framework

import (
	"github.com/Azure/placement-policy-scheduler-plugins/api/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"

	schedv1alpha1 "sigs.k8s.io/scheduler-plugins/pkg/apis/scheduling/v1alpha1"
)

const (
	// NamespaceKubeSystem is the name of kube-system namespace.
	NamespaceKubeSystem = "kube-system"
)

// TryAddDefaultSchemes tries to add various schemes.
func TryAddDefaultSchemes(scheme *runtime.Scheme) {
	// Add the core schemes.
	_ = corev1.AddToScheme(scheme)

	// Add the apps schemes.
	_ = appsv1.AddToScheme(scheme)

	// Add the api extensions (CRD) to the scheme.
	_ = apiextensionsv1beta.AddToScheme(scheme)
	_ = apiextensionsv1.AddToScheme(scheme)

	// Add kubescheduerl to the scheme.
	_ = schedv1alpha1.AddToScheme(scheme)

	// Add placedment-policy-scheduler-plugins v1alpha1 to the scheme
	_ = v1alpha1.AddToScheme(scheme)
}
