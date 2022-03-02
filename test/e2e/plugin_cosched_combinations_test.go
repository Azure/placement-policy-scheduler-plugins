//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestMustStrictCoscheduling(t *testing.T) {
	deploymentFeat := features.New("Test Must Strict Placement policy with Coscheduling plugins").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// deploy placement policy config
			if err := deploySchedulerConfig(cfg.KubeconfigFile(), cfg.Namespace(), "examples", "v1alpha1_placementpolicy_strict_must.yml"); err != nil {
				t.Error("Failed to deploy placement policy config", err)
			}

			lables := map[string]string{
				"app":                              "nginx",
				"pod-group.scheduling.sigs.k8s.io": "nginx",
			}

			wd, err := os.Getwd()
			if err != nil {
				t.Error(err)
			}
			pluginsResourceAbsolutePath, err := filepath.Abs(filepath.Join(wd, "plugins/coscheduling"))
			if err != nil {
				t.Error(err)
			}

			// deploy Coscheduling config
			if err := KubectlApply(cfg.KubeconfigFile(), "scheduler-plugins", []string{"-f", fmt.Sprintf("%s/%s", pluginsResourceAbsolutePath, "")}); err != nil {
				t.Error("Failed to deploy coscheduling config", err)
			}

			// deploy a sample replicaset
			statefulset := newStatefulSet(cfg.Namespace(), "statefulset-test", 6, lables)
			if err := cfg.Client().Resources().Create(ctx, statefulset); err != nil {
				t.Error("Failed to create statefulset", err)
			}

			return ctx
		}).
		Assess("Pods successfully assigned to the right nodes with Must Strict and Coscheduling plugins option", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Error("Failed to create new client", err)
			}
			resultStatefulset := appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: "statefulset-test", Namespace: cfg.Namespace()},
			}

			if err := wait.For(conditions.New(client.Resources()).ResourceMatch(&resultStatefulset, func(object k8s.Object) bool {
				s := object.(*appsv1.StatefulSet)
				return s.Status.ReadyReplicas == 3
			}), wait.WithTimeout(time.Minute*2)); err != nil {
				t.Error("Failed to deploy a statefulset", err)
			}

			var pods corev1.PodList
			if err := client.Resources().List(ctx, &pods, resources.WithLabelSelector(labels.FormatLabels(map[string]string{"app": "nginx", "pod-group.scheduling.sigs.k8s.io": "nginx"}))); err != nil {
				t.Error("cannot get list of pods", err)
			}

			for i := range pods.Items {
				if pods.Items[i].Spec.NodeName != "placement-policy-worker3" {
					continue
				} else {
					t.Error("pods assigned to the wrong node", err)
				}
			}

			return context.WithValue(ctx, "statefulset-test", &resultStatefulset)
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Error("failed to create new Client", err)
			}
			dep := ctx.Value("statefulset-test").(*appsv1.StatefulSet)
			if err := client.Resources().Delete(ctx, dep); err != nil {
				t.Error("failed to delete Statefulset", err)
			}
			return ctx
		}).Feature()
	testenv.Test(t, deploymentFeat)
}
