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
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestMustStrictNoderesources(t *testing.T) {
	deploymentFeat := features.New("Test Must Strict Placement policy with noderesources plugins").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			wd, err := os.Getwd()
			if err != nil {
				t.Error(err)
			}
			pluginsResourceAbsolutePath, err := filepath.Abs(filepath.Join(wd, "plugins/noderesources"))
			if err != nil {
				t.Error(err)
			}
			// deploy noderesources config
			if err := KubectlApply(cfg.KubeconfigFile(), "kube-system", []string{"-f", fmt.Sprintf("%s/%s", pluginsResourceAbsolutePath, "noderesources.yaml")}); err != nil {
				t.Error("Failed to deploy config", err)
			}

			// deploy placement policy config
			if err := deploySchedulerConfig(cfg.KubeconfigFile(), cfg.Namespace(), "examples", "v1alpha1_placementpolicy_strict_must.yml"); err != nil {
				t.Error("Failed to deploy config", err)
			}

			lables := map[string]string{
				"name": "test",
			}
			// deploy a sample replicaset
			deployment := newNodeResourceDeployment(cfg.Namespace(), "deployment-test", 1, lables)
			if err := cfg.Client().Resources().Create(ctx, deployment); err != nil {
				t.Error("Failed to create deployment", err)
			}

			return ctx
		}).
		Assess("Pods successfully assigned to the right nodes with Must Strict and noderesources plugins option", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Error("Failed to create new client", err)
			}
			resultDeployment := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "deployment-test", Namespace: cfg.Namespace()},
			}

			if err = wait.For(conditions.New(client.Resources()).DeploymentConditionMatch(&resultDeployment, appsv1.DeploymentAvailable, corev1.ConditionTrue),
				wait.WithTimeout(time.Minute*2)); err != nil {
				t.Error("deployment not found", err)
			}

			var pods corev1.PodList
			if err := client.Resources().List(ctx, &pods, resources.WithLabelSelector(labels.FormatLabels(map[string]string{"name": "test"}))); err != nil {
				t.Error("cannot get list of pods", err)
			}

			for i := range pods.Items {
				if pods.Items[i].Spec.NodeName != "placement-policy-worker3" {
					continue
				} else {
					t.Error("pods assigned to the wrong node", err)
				}
			}
			return context.WithValue(ctx, "deployment-test", &resultDeployment)
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Error("failed to create new Client", err)
			}
			dep := ctx.Value("deployment-test").(*appsv1.Deployment)
			if err := client.Resources().Delete(ctx, dep); err != nil {
				t.Error("failed to delete deployment", err)
			}
			return ctx
		}).Feature()
	testenv.Test(t, deploymentFeat)
}
