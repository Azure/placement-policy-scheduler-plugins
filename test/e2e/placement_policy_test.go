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

const deploymentTest = "deployment-test"

var (
	nodeSelectorLabels = map[string]string{"node": "want"}
	podSelectorLabels  = map[string]string{"app": "nginx"}
)

func TestDifferentDeploymentObjects(t *testing.T) {
	deploymentFeat := features.New("Test deployment").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// deploy placement policy config
			if err := deploySchedulerConfig(cfg.KubeconfigFile(), cfg.Namespace(), "examples", "v1alpha1_placementpolicy_strict_must.yml"); err != nil {
				t.Error("Failed to deploy config", err)
			}

			deployment := newDeployment(cfg.Namespace(), deploymentTest, 1, podSelectorLabels)
			if err := cfg.Client().Resources().Create(ctx, deployment); err != nil {
				t.Error("Failed to create the deployment", err)
			}
			return ctx
		}).
		Assess("Deployment successfully assigned to the right nodes", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Error("Failed to create new client", err)
			}
			resultDeployment := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: deploymentTest, Namespace: cfg.Namespace()},
			}

			if err = wait.For(conditions.New(client.Resources()).DeploymentConditionMatch(&resultDeployment, appsv1.DeploymentAvailable, corev1.ConditionTrue),
				wait.WithTimeout(time.Minute*2)); err != nil {
				t.Error("deployment not found", err)
			}

			return context.WithValue(ctx, deploymentTest, &resultDeployment)
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Error("Failed to create new client", err)
			}
			dep := ctx.Value(deploymentTest).(*appsv1.Deployment)
			if err := client.Resources().Delete(ctx, dep); err != nil {
				t.Error("Failed to delete the deployment", err)
			}
			return ctx
		}).Feature()

	statefulsetFeat := features.New("Test statefulset").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// deploy placement policy config
			if err := deploySchedulerConfig(cfg.KubeconfigFile(), cfg.Namespace(), "examples", "v1alpha1_placementpolicy_strict_must.yml"); err != nil {
				t.Error(err)
			}

			statefulset := newStatefulSet(cfg.Namespace(), "statefulset-test", 1, podSelectorLabels)
			if err := cfg.Client().Resources().Create(ctx, statefulset); err != nil {
				t.Error("Failed to create statefulset", err)
			}
			return ctx
		}).
		Assess("Statefulset successfully assigned to the right nodes", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Error("Failed to create new client", err)
			}
			resultStatefulset := appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: "statefulset-test", Namespace: cfg.Namespace()},
			}

			if err := wait.For(conditions.New(client.Resources()).ResourceMatch(&resultStatefulset, func(object k8s.Object) bool {
				s := object.(*appsv1.StatefulSet)
				return float64(s.Status.ReadyReplicas)/float64(*s.Spec.Replicas) >= 1
			}), wait.WithTimeout(time.Minute*2)); err != nil {
				t.Error("Failed to deploy a statefulset", err)
			}

			return context.WithValue(ctx, "statefulset-test", &resultStatefulset)
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Error("Failed to create new Client", err)
			}
			st := ctx.Value("statefulset-test").(*appsv1.StatefulSet)
			if err := client.Resources().Delete(ctx, st); err != nil {
				t.Error("Failed to delete statefulset", err)
			}
			return ctx
		}).Feature()

	testenv.Test(t, deploymentFeat, statefulsetFeat)
}

func TestMustStrictDeployment(t *testing.T) {
	deploymentFeat := features.New("Test Must Strict Placement policy").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// deploy placement policy config
			if err := deploySchedulerConfig(cfg.KubeconfigFile(), cfg.Namespace(), "examples", "v1alpha1_placementpolicy_strict_must.yml"); err != nil {
				t.Error("Failed to deploy config", err)
			}

			deployment := newDeployment(cfg.Namespace(), deploymentTest, 10, podSelectorLabels)
			if err := cfg.Client().Resources().Create(ctx, deployment); err != nil {
				t.Error("Failed to create deployment", err)
			}
			return ctx
		}).
		Assess("Pods successfully assigned to the right nodes with Must Strict option", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Error("Failed to create new client", err)
			}
			resultDeployment := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: deploymentTest, Namespace: cfg.Namespace()},
			}

			if err = wait.For(conditions.New(client.Resources()).DeploymentConditionMatch(&resultDeployment, appsv1.DeploymentAvailable, corev1.ConditionTrue),
				wait.WithTimeout(time.Minute*2)); err != nil {
				t.Error("deployment not found", err)
			}

			// check if 4 pods out of 10 (40%) are running in the node with the same node selector
			pods := &corev1.PodList{}
			if err := wait.For(conditions.New(client.Resources()).ResourceListMatchN(pods, 4, func(object k8s.Object) bool {

				if object.(*corev1.Pod).Spec.NodeName != "placement-policy-worker3" {
					return true
				}
				return false
			}, resources.WithLabelSelector(labels.FormatLabels(podSelectorLabels))),
				wait.WithTimeout(time.Minute*4)); err != nil {
				t.Error("number of pods assigned to nodes with the required nodeSelector do not match", err)
			}

			return context.WithValue(ctx, deploymentTest, &resultDeployment)
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Error("failed to create new Client", err)
			}
			dep := ctx.Value(deploymentTest).(*appsv1.Deployment)
			if err := client.Resources().Delete(ctx, dep); err != nil {
				t.Error("failed to delete deployment", err)
			}
			return ctx
		}).Feature()
	testenv.Test(t, deploymentFeat)
}

func TestMustBestEffortDeployment(t *testing.T) {
	deploymentFeat := features.New("Test Must BestEffort Placement policy").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// deploy placement policy config
			if err := deploySchedulerConfig(cfg.KubeconfigFile(), cfg.Namespace(), "examples", "v1alpha1_placementpolicy_must_besteffort.yml"); err != nil {
				t.Error(err)
			}

			deployment := newDeployment(cfg.Namespace(), deploymentTest, 3, podSelectorLabels)
			if err := cfg.Client().Resources().Create(ctx, deployment); err != nil {
				t.Error("Failed to create deployment", err)
			}
			return ctx
		}).
		Assess("Deployment successfully assigned to the right nodes with Must BestEffort option", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Error("Failed to create new client", err)
			}
			resultDeployment := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: deploymentTest, Namespace: cfg.Namespace()},
			}

			if err = wait.For(conditions.New(client.Resources()).DeploymentConditionMatch(&resultDeployment, appsv1.DeploymentAvailable, corev1.ConditionTrue),
				wait.WithTimeout(time.Minute*2)); err != nil {
				t.Error("some or all replicas in the deployment failed", err)
			}

			return context.WithValue(ctx, deploymentTest, &resultDeployment)
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Error("Failed to create new Client", err)
			}
			dep := ctx.Value(deploymentTest).(*appsv1.Deployment)
			if err := client.Resources().Delete(ctx, dep); err != nil {
				t.Error("Failed to delete deployment", err)
			}
			return ctx
		}).Feature()
	testenv.Test(t, deploymentFeat)
}

func TestMustNotStrictDeployment(t *testing.T) {
	deploymentFeat := features.New("Test MustNot Placement policy Action").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// deploy placement policy config
			if err := deploySchedulerConfig(cfg.KubeconfigFile(), cfg.Namespace(), "examples", "v1alpha1_placementpolicy_strict_mustnot.yml"); err != nil {
				t.Error(err)
			}

			deployment := newDeployment(cfg.Namespace(), deploymentTest, 1, podSelectorLabels)
			if err := cfg.Client().Resources().Create(ctx, deployment); err != nil {
				t.Error("Failed to create deployment", err)
			}
			return ctx
		}).
		Assess("Deployment successfully assigned to the right nodes with MustNot Action", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Error("Failed to create new client", err)
			}
			resultDeployment := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: deploymentTest, Namespace: cfg.Namespace()},
			}

			//check if 4 pods out of 10 (40%) are not running in the node with the same node selector
			pods := &corev1.PodList{}
			if err := wait.For(conditions.New(client.Resources()).ResourceListMatchN(pods, 4, func(object k8s.Object) bool {

				if object.(*corev1.Pod).Spec.NodeName == "placement-policy-worker3" {
					return true
				}
				return false
			}, resources.WithLabelSelector(labels.FormatLabels(podSelectorLabels))),
				wait.WithTimeout(time.Minute*4)); err != nil {
				t.Error("Pod assigned to the wrong node", err)
			}

			return context.WithValue(ctx, deploymentTest, &resultDeployment)
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Error("Failed to create new Client", err)
			}
			dep := ctx.Value(deploymentTest).(*appsv1.Deployment)
			if err := client.Resources().Delete(ctx, dep); err != nil {
				t.Error("Failed to delete deployment", err)
			}
			return ctx
		}).Feature()
	testenv.Test(t, deploymentFeat)
}

func TestConcurrentDeployments(t *testing.T) {

	firstDeployment := "multidep1-test"
	secondDeployment := "multidep2-test"

	deploymentFeat := features.New("Test concurrent deployment").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {

			// deploy placement policy config
			if err := deploySchedulerConfig(cfg.KubeconfigFile(), cfg.Namespace(), "examples", "v1alpha1_placementpolicy_strict_must.yml"); err != nil {
				t.Error(err)
			}

			// Create list of deployments (multiple pods)
			deployment := newDeployment(cfg.Namespace(), firstDeployment, 5, podSelectorLabels)
			if err := cfg.Client().Resources().Create(ctx, deployment); err != nil {
				t.Error("Failed to create deployment", err)
			}
			deployment2 := newDeployment(cfg.Namespace(), secondDeployment, 5, podSelectorLabels)
			if err := cfg.Client().Resources().Create(ctx, deployment2); err != nil {
				t.Error("Failed to create deployment", err)
			}
			return ctx
		}).
		Assess("Pods successfully assigned to the right nodes", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// check list of pods already assigned to the pods
			client, err := cfg.NewClient()
			if err != nil {
				t.Error("Failed to create new client", err)
			}
			resultDeployment1 := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: firstDeployment, Namespace: cfg.Namespace()},
			}
			if err = wait.For(conditions.New(client.Resources()).DeploymentConditionMatch(&resultDeployment1, appsv1.DeploymentAvailable, corev1.ConditionTrue),
				wait.WithTimeout(time.Minute*3)); err != nil {
				t.Error("fialed to find the first deployment", err)
			}

			resultDeployment2 := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: secondDeployment, Namespace: cfg.Namespace()},
			}
			if err = wait.For(conditions.New(client.Resources()).DeploymentConditionMatch(&resultDeployment2, appsv1.DeploymentAvailable, corev1.ConditionTrue),
				wait.WithTimeout(time.Minute*3)); err != nil {
				t.Error("fialed to find the second deployment", err)
			}
			if err := client.Resources().Delete(ctx, &resultDeployment1); err != nil {
				t.Error("fialed to delete the first deployment", err)
			}
			if err := client.Resources().Delete(ctx, &resultDeployment2); err != nil {
				t.Error("fialed to delete the second deployment", err)
			}
			return ctx
		}).Feature()

	statefulsetFeat := features.New("Test concurrent statefulset").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// deploy placement policy config
			if err := deploySchedulerConfig(cfg.KubeconfigFile(), cfg.Namespace(), "examples", "v1alpha1_placementpolicy_strict_must.yml"); err != nil {
				t.Error(err)
			}

			statefulset := newStatefulSet(cfg.Namespace(), "statefulset-test", 5, podSelectorLabels)
			if err := cfg.Client().Resources().Create(ctx, statefulset); err != nil {
				t.Error("Failed to create statefulset", err)
			}
			return ctx
		}).
		Assess("Statefulset successfully assigned to the right nodes", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Error("Failed to create new client", err)
			}
			resultStatefulset := appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: "statefulset-test", Namespace: cfg.Namespace()},
			}

			if err := wait.For(conditions.New(client.Resources()).ResourceMatch(&resultStatefulset, func(object k8s.Object) bool {
				s := object.(*appsv1.StatefulSet)
				return float64(s.Status.ReadyReplicas)/float64(*s.Spec.Replicas) >= 1
			}), wait.WithTimeout(time.Minute*2)); err != nil {
				t.Error("Failed to deploy a statefulset", err)
			}

			return context.WithValue(ctx, "statefulset-test", &resultStatefulset)
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Error("Failed to create new Client", err)
			}
			st := ctx.Value("statefulset-test").(*appsv1.StatefulSet)
			if err := client.Resources().Delete(ctx, st); err != nil {
				t.Error("Failed to delete deployment", err)
			}
			return ctx
		}).Feature()

	testenv.Test(t, deploymentFeat, statefulsetFeat)
}

// deploy placement policy config
func deploySchedulerConfig(kubeConfig, namespace, resourcePath, fileName string) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	exampleResourceAbsolutePath, err := filepath.Abs(filepath.Join(wd, "/../../", resourcePath))
	if err != nil {
		return err
	}
	if err := KubectlApply(kubeConfig, namespace, []string{"-f", filepath.Join(exampleResourceAbsolutePath, fileName)}); err != nil {
		return err
	}

	return nil
}
