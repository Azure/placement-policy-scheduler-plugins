package e2e

import (
	"testing"

	"github.com/Azure/placement-policy-scheduler-plugins/test/e2e/framework"

	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "placementpolicy")
}

var _ = BeforeSuite(func() {
	// Ensure cluster is running and ready before starting tests
	By("Checking KinD Cluster is up and ready")
	By("Configuring Placement Policy plugins")
})

var _ = AfterSuite(func() {
	// cleanup
	getPodsLogs()
})

func initScheme() *runtime.Scheme {
	sc := runtime.NewScheme()
	framework.TryAddDefaultSchemes(sc)
	return sc
}

func getPodsLogs() {
	//TODO
}
