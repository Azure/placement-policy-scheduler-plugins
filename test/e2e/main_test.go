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

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
)

var (
	providerResourceDirectory = "manifest_staging/deploy"
	providerResource          = "kube-scheduler-configuration.yml"
	testenv                   env.Environment
)

func TestMain(m *testing.M) {
	testenv = env.NewWithConfig(envconf.New())
	// Create KinD Cluster
	kindClusterName := envconf.RandomName("placement-policy", 16)
	namespace := envconf.RandomName("pp-ns", 16)
	testenv.Setup(
		envfuncs.CreateKindClusterWithConfig(kindClusterName, "kindest/node:v1.22.2", "kind-config.yaml"),
		envfuncs.CreateNamespace(namespace),
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
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
			time.Sleep(5 * time.Second)
			return ctx, nil
		},
	).Finish( // Cleanup KinD Cluster
		envfuncs.DeleteNamespace(namespace),
		envfuncs.DestroyKindCluster(kindClusterName),
	)

	os.Exit(testenv.Run(m))
}
