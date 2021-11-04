package e2e_test

import (
	"testing"

	"github.com/Azure/placement-policy-scheduler-plugins/test/e2e/framework"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "placementpolicy")
}

var _ = BeforeSuite(func() {

})

var _ = AfterSuite(func() {
	// cleanup
})

func initScheme() *runtime.Scheme {
	sc := runtime.NewScheme()
	framework.TryAddDefaultSchemes(sc)
	return sc
}
