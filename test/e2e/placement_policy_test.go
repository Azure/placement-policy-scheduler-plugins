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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestAssignDifferentDeployments(t *testing.T) {

	deploymentFeat := features.New("Test deployment").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// deploy placement policy config
			wd, err := os.Getwd()
			if err != nil {
				klog.ErrorS(err, "Failed to find folder path")
				return ctx
			}

			exampleResourceAbsolutePath, err := filepath.Abs(filepath.Join(wd, "/../../", "examples"))
			if err != nil {
				klog.ErrorS(err, "Failed to resource file")
				return ctx
			}
			if err := KubectlApply(cfg.KubeconfigFile(), cfg.Namespace(), []string{"-f", filepath.Join(exampleResourceAbsolutePath, "v1alpha1_placementpolicy.yml")}); err != nil {
				klog.ErrorS(err, "Failed to deploy the placement policy scheduler config")
				return ctx
			}

			labels := map[string]string{"node": "want"}
			deployment := NewDeployment(cfg.Namespace(), "deployment-test", 1, labels)
			if err := cfg.Client().Resources().Create(ctx, deployment); err != nil {
				klog.ErrorS(err, "Failed to create the deployment")
				return ctx
			}

			return ctx
		}).
		Assess("Deployment successfully assigned to the right nodes", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				klog.ErrorS(err, "Failed to create new client")
				return ctx
			}
			resultDeployment := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "deployment-test", Namespace: cfg.Namespace()},
			}

			err = wait.For(conditions.New(client.Resources()).ResourceMatch(&resultDeployment, func(object k8s.Object) bool {
				d := object.(*appsv1.Deployment)
				return float64(d.Status.ReadyReplicas)/float64(*d.Spec.Replicas) >= 1
			}), wait.WithTimeout(time.Minute*1))

			if err != nil {
				t.Error("deployment not found")
			}

			return context.WithValue(ctx, "deployment-test", &resultDeployment)
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				klog.ErrorS(err, "Failed to create new client")
				return ctx
			}
			dep := ctx.Value("deployment-test").(*appsv1.Deployment)
			if err := client.Resources().Delete(ctx, dep); err != nil {
				klog.ErrorS(err, "Failed to delete the deployment")
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
