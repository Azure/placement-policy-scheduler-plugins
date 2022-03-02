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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
)

var (
	providerResourceDirectory = "manifest_staging/deploy"
	providerResource          = "kube-scheduler-configuration.yml"
	testenv                   env.Environment
	registry                  = os.Getenv("REGISTRY")
	imageName                 = os.Getenv("IMAGE_NAME")
	imageVersion              = os.Getenv("IMAGE_VERSION")
)

func TestMain(m *testing.M) {
	testenv = env.NewWithConfig(envconf.New())
	// Create KinD Cluster
	kindClusterName := envconf.RandomName("placement-policy", 16)
	namespace := envconf.RandomName("pp-ns", 16)
	testenv.Setup(
		envfuncs.CreateKindClusterWithConfig(kindClusterName, "kindest/node:v1.22.2", "kind-config.yaml"),
		envfuncs.CreateNamespace(namespace),
		envfuncs.LoadDockerImageToCluster(kindClusterName, fmt.Sprintf("%s/%s:%s", registry, imageName, imageVersion)),
		deploySchedulerManifest(),
	).Finish( // Cleanup KinD Cluster
		envfuncs.DeleteNamespace(namespace),
		envfuncs.DestroyKindCluster(kindClusterName),
	)
	os.Exit(testenv.Run(m))
}

//deploy placement policy manifest
func deploySchedulerManifest() env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		wd, err := os.Getwd()
		if err != nil {
			return ctx, err
		}
		providerResourceAbsolutePath, err := filepath.Abs(filepath.Join(wd, "/../../", providerResourceDirectory))
		if err != nil {
			return ctx, err
		}
		// start a CRD deployment
		if err := KubectlApply(cfg.KubeconfigFile(), "kube-system", []string{"-f", fmt.Sprintf("%s/%s", providerResourceAbsolutePath, providerResource)}); err != nil {
			return ctx, err
		}
		// wait for the deployment to finish becoming available
		dep := appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "pp-plugins-scheduler", Namespace: "kube-system"},
		}

		client, err := cfg.NewClient()
		if err != nil {
			klog.ErrorS(err, "Failed to create new Client")
			return ctx, err
		}

		if err := wait.For(conditions.New(client.Resources()).ResourceMatch(&dep, func(object k8s.Object) bool {
			d := object.(*appsv1.Deployment)
			return float64(d.Status.ReadyReplicas)/float64(*d.Spec.Replicas) >= 1
		}), wait.WithTimeout(time.Minute*1)); err != nil {

			klog.ErrorS(err, " Failed to deploy placement policy scheduler")
			return ctx, err
		}

		return ctx, nil
	}
}
