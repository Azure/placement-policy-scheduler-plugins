package integration

import (
	"context"
	"os"
	"testing"
	"time"

	v1 "github.com/Azure/placement-policy-scheduler-plugins/api/v1alpha1"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	st "k8s.io/kubernetes/pkg/scheduler/testing"
	testutil "k8s.io/kubernetes/test/integration/util"

	"sigs.k8s.io/scheduler-plugins/pkg/apis/scheduling"
	"sigs.k8s.io/scheduler-plugins/test/util"
)

func TestPlacementPolicyPlugins(t *testing.T) {

	todo := context.TODO()
	ctx, cancelFunc := context.WithCancel(todo)
	testCtx := &testutil.TestContext{
		Ctx:      ctx,
		CancelFn: cancelFunc,
		CloseFn:  func() {},
	}

	t.Log("create apiserver")
	_, config := util.StartApi(t, todo.Done())

	config.ContentType = "application/json"

	apiExtensionClient, err := apiextensionsclient.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}

	kubeConfigPath := util.BuildKubeConfigFile(config)
	if len(kubeConfigPath) == 0 {
		t.Fatal("Build KubeConfigFile failed")
	}
	defer os.RemoveAll(kubeConfigPath)

	t.Log("create crd")
	if _, err := apiExtensionClient.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, makeCRD(), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	cs := kubernetes.NewForConfigOrDie(config)

	runtime.Must(v1.AddToScheme(scheme.Scheme))
	if err = wait.Poll(100*time.Millisecond, 3*time.Second, func() (done bool, err error) {
		groupList, _, err := cs.ServerGroupsAndResources()
		if err != nil {
			return false, nil
		}
		for _, group := range groupList {
			if group.Name == scheduling.GroupName {
				return true, nil
			}
		}
		t.Log("waiting for crd api ready")
		return false, nil
	}); err != nil {
		t.Fatalf("Waiting for crd read time out: %v", err)
	}

	// Create a Node.
	nodeName := "fake-node"
	node := st.MakeNode().Name(nodeName).Label("node", nodeName).Obj()

	node, err = cs.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create Node %q: %v", nodeName, err)
	}

	testCtx.ClientSet = cs
	testCtx = util.InitTestSchedulerWithOptions(
		t,
		testCtx,
		true,
		//scheduler.WithProfiles(profile),
		//scheduler.WithFrameworkOutOfTreeRegistry(registry),
	)
	t.Log("init scheduler success")
	defer testutil.CleanupTest(t, testCtx)
}

func makeCRD() *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "placementpolicies.placement-policy.scheduling.x-k8s.io",
			Annotations: map[string]string{
				"controller-gen.kubebuilder.io/version": "v0.7.0",
			},
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: scheduling.GroupName,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1alpha1", Served: true, Storage: true,
				Schema: &apiextensionsv1.CustomResourceValidation{
					OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
						Type: "object",
						Properties: map[string]apiextensionsv1.JSONSchemaProps{
							"spec": {
								Type: "object",
								Properties: map[string]apiextensionsv1.JSONSchemaProps{
									"enforcementMode": {
										Type: "string",
									},
									"nodeSelector": {
										Type: "object",
									},
									"PodSelector": {
										Type: "object",
									},
									"Weight": {
										Type: "integer",
									},
									"Policy": {
										Type: "object",
										Properties: map[string]apiextensionsv1.JSONSchemaProps{
											"Action": {
												Type: "string",
											},
											"TargetSize": {
												//AnyOf: ["integer", "string"],
											},
										},
									},
								},
							},
						},
					},
				}}},
			Scope: apiextensionsv1.NamespaceScoped,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:       "PlacementPolicy",
				Plural:     "PlacementPolicies",
				ShortNames: []string{"pp", "pps"},
			},
		},
	}
}
