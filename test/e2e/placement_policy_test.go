//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestAssignDifferentDeployments(t *testing.T) {

	deploymentFeat := features.New("Test deployment").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// deploy placement policy config
			wd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}

			exampleResourceAbsolutePath, err := filepath.Abs(filepath.Join(wd, "/../../", "examples"))
			if err != nil {
				t.Fatal(err)
			}
			if err := KubectlApply(cfg.KubeconfigFile(), cfg.Namespace(), []string{"-f", filepath.Join(exampleResourceAbsolutePath, "v1alpha_placementpolicy.yml")}); err != nil {
				t.Fatal(err)
			}
			time.Sleep(5 * time.Second)
			labels := map[string]string{"node": "want"}

			deployment := NewDeployment(cfg.Namespace(), "deployment-test", 1, labels)
			if err := cfg.Client().Resources().Create(ctx, deployment); err != nil {
				t.Fatal(err)
			}
			time.Sleep(2 * time.Second)
			return ctx
		}).
		Assess("Deployment successfully assigned to the right nodes", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var resultDeployment appsv1.Deployment
			if err := cfg.Client().Resources().Get(ctx, "deployment-test", cfg.Namespace(), &resultDeployment); err != nil {
				klog.ErrorS(err, "Deployment Failed!")
			}

			if &resultDeployment != nil {
				t.Logf("deployment found: %s", resultDeployment.Name)
			}
			return context.WithValue(ctx, "deployment-test", &resultDeployment)
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			dep := ctx.Value("deployment-test").(*appsv1.Deployment)
			if err := client.Resources().Delete(ctx, dep); err != nil {
				t.Fatal(err)
			}
			return ctx
		}).Feature()

	daemonsetFeat := features.New("Test daemonset").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Create a daemonset
			return ctx
		}).
		Assess("Deployment successfully assigned to the right nodes", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// check pod already assigned to the right node
			return ctx
		}).Feature()

	statefulsetFeat := features.New("Test statefulset").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Create a statefulset
			return ctx
		}).
		Assess("Deployment successfully assigned to the right nodes", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// check pod already assigned to the right node
			return ctx
		}).Feature()

	testenv.Test(t, deploymentFeat, daemonsetFeat, statefulsetFeat)
}

func TestConcurrentDeployments(t *testing.T) {

	deploySchedFeat := features.New("Test concurrent deployment").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Create list of deployments (multiple pods)
			return ctx
		}).
		Assess("Deployment successfully assigned to the right nodes", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// check list of pods already assigned to the pods
			return ctx
		}).Feature()

	testenv.Test(t, deploySchedFeat)
}
