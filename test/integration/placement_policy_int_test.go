package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Azure/placement-policy-scheduler-plugins/apis/v1alpha1"
	"github.com/Azure/placement-policy-scheduler-plugins/pkg/client/clientset/versioned"
	"github.com/Azure/placement-policy-scheduler-plugins/pkg/plugins/placementpolicy"

	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	apiservertesting "k8s.io/kubernetes/cmd/kube-apiserver/app/testing"
	"k8s.io/kubernetes/pkg/scheduler"

	schedapi "k8s.io/kubernetes/pkg/scheduler/apis/config"
	fwkruntime "k8s.io/kubernetes/pkg/scheduler/framework/runtime"
	st "k8s.io/kubernetes/pkg/scheduler/testing"
	testfwk "k8s.io/kubernetes/test/integration/framework"
	testutil "k8s.io/kubernetes/test/integration/util"
	imageutils "k8s.io/kubernetes/test/utils/image"
	"sigs.k8s.io/yaml"
)

const (
	node1 = "fake-node-1"
	node2 = "fake-node-2"
	node3 = "fake-node-3"
)

func TestPlacementPolicyPlugins(t *testing.T) {

	t.Log("Creating API Server...")
	// Start API Server with apiextensions supported.
	server := apiservertesting.StartTestServerOrDie(
		t, apiservertesting.NewDefaultTestServerOptions(),
		[]string{"--disable-admission-plugins=ServiceAccount,TaintNodesByCondition,Priority", "--runtime-config=api/all=true"},
		testfwk.SharedEtcd(),
	)

	todo := context.TODO()
	ctx, cancelFunc := context.WithCancel(todo)
	testCtx := &testutil.TestContext{
		Ctx:      ctx,
		CancelFn: cancelFunc,
		CloseFn:  func() {},
	}

	t.Log("Creating CRD...")
	apiExtensionClient := apiextensionsclient.NewForConfigOrDie(server.ClientConfig)
	if _, err := apiExtensionClient.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, makePlacementPolicyCRD(), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	server.ClientConfig.ContentType = "application/json"
	testCtx.KubeConfig = server.ClientConfig
	cs := kubernetes.NewForConfigOrDie(testCtx.KubeConfig)
	testCtx.ClientSet = cs
	extClient := versioned.NewForConfigOrDie(testCtx.KubeConfig)

	if err := wait.Poll(100*time.Millisecond, 3*time.Second, func() (done bool, err error) {
		groupList, _, err := cs.ServerGroupsAndResources()
		if err != nil {
			return false, nil
		}
		for _, group := range groupList {
			if group.Name == v1alpha1.GroupName {
				t.Log("The CRD is ready to serve")
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		t.Fatalf("Timed out waiting for CRD to be ready: %v", err)
	}

	cfg, err := NewDefaultSchedulerComponentConfig()
	if err != nil {
		t.Fatal(err)
	}

	cfg.Profiles[0].Plugins.PreFilter.Enabled = append(cfg.Profiles[0].Plugins.PreFilter.Enabled, schedapi.Plugin{Name: placementpolicy.Name})
	cfg.Profiles[0].Plugins.Filter.Enabled = append(cfg.Profiles[0].Plugins.Filter.Enabled, schedapi.Plugin{Name: placementpolicy.Name})
	cfg.Profiles[0].Plugins.PreScore.Enabled = append(cfg.Profiles[0].Plugins.PreScore.Enabled, schedapi.Plugin{Name: placementpolicy.Name})
	cfg.Profiles[0].Plugins.Score.Enabled = append(cfg.Profiles[0].Plugins.Score.Enabled, schedapi.Plugin{Name: placementpolicy.Name})

	testCtx = InitTestSchedulerWithOptions(
		t,
		testCtx,
		true,
		scheduler.WithKubeConfig(server.ClientConfig),
		scheduler.WithProfiles(cfg.Profiles...),
		scheduler.WithFrameworkOutOfTreeRegistry(fwkruntime.Registry{placementpolicy.Name: placementpolicy.New}),
	)
	t.Log("Init scheduler success")
	defer testutil.CleanupTest(t, testCtx)

	ns, err := cs.CoreV1().Namespaces().Create(testCtx.Ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{GenerateName: "integration-test-"}}, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("Failed to create integration test ns: %v", err)
	}

	// Create nodes.

	for _, nodeName := range []string{node1, node2} {
		newNode := st.MakeNode().Name(nodeName).Label("node", "want").Obj()
		n, err := cs.CoreV1().Nodes().Create(ctx, newNode, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Failed to create Node %q: %v", nodeName, err)
		}
		t.Logf(" Node %s created: %v", nodeName, n)
	}
	unwantNode := st.MakeNode().Name(node3).Label("node", "unwant").Obj()
	n, err := cs.CoreV1().Nodes().Create(ctx, unwantNode, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create Node %q: %v", node3, err)
	}
	t.Logf(" Node %s created: %v", node3, n)

	busyBox := imageutils.GetE2EImage(imageutils.BusyBox)

	for _, pp := range []struct {
		name          string
		pods          []*v1.Pod
		policy        *v1alpha1.PlacementPolicy
		expectedNodes []string
	}{
		{
			name: "Case of Placement Policy StrictMust",
			pods: []*v1.Pod{
				st.MakePod().Namespace(ns.Name).Name("policymust-1").Label("app", "nginx").Container(busyBox).ZeroTerminationGracePeriod().Obj(),
				st.MakePod().Namespace(ns.Name).Name("policymust-2").Label("app", "nginx").Container(busyBox).ZeroTerminationGracePeriod().Obj(),
				st.MakePod().Namespace(ns.Name).Name("policymust-3").Label("app", "nginx").Container(busyBox).ZeroTerminationGracePeriod().Obj(),
				st.MakePod().Namespace(ns.Name).Name("policymust-4").Label("app", "nginx").Container(busyBox).ZeroTerminationGracePeriod().Obj(),
				st.MakePod().Namespace(ns.Name).Name("policymust-5").Label("app", "nginx").Container(busyBox).ZeroTerminationGracePeriod().Obj(),
				st.MakePod().Namespace(ns.Name).Name("policymust-6").Label("app", "else").Container(busyBox).ZeroTerminationGracePeriod().Obj(),
			},
			policy:        MakePlacementPolicy(v1alpha1.EnforcementModeStrict, intstr.FromString("80%"), v1alpha1.ActionMust, "strictmust", ns.Name),
			expectedNodes: []string{node1, node2},
		},
	} {
		t.Run(pp.name, func(t *testing.T) {
			t.Logf("Start-placementpolicy-test %v", pp.name)
			defer deletePlacementPolicy(ctx, extClient, *pp.policy)
			// create pod group
			if err := createPlacementPolicy(ctx, extClient, pp.policy); err != nil {
				t.Fatal(err)
			}
			defer testutil.CleanupPods(cs, t, pp.pods)
			// Create Pods, we will expect them to be scheduled in a reversed order.
			for i := range pp.pods {
				klog.InfoS("Creating pod ", "podName", pp.pods[i].Name)
				if _, err := cs.CoreV1().Pods(pp.pods[i].Namespace).Create(testCtx.Ctx, pp.pods[i], metav1.CreateOptions{}); err != nil {
					t.Fatalf("Failed to create Pod %q: %v", pp.pods[i].Name, err)
				}
			}

			targetCount := 0
			for _, p := range pp.pods {
				// Wait for the pod to be scheduled.
				err = wait.Poll(1*time.Second, 20*time.Second, func() (bool, error) {
					return PodScheduled(cs, ns.Name, p.Name), nil
				})
				if err != nil {
					t.Errorf("pod %q to be scheduled, error: %v", p.Name, err)
				}

				t.Logf("pod scheduled: %v", p.Name)
			}
			podList, err := cs.CoreV1().Pods(ns.Name).List(testCtx.Ctx, metav1.ListOptions{LabelSelector: "app=nginx"})
			if err != nil {
				t.Fatal(err)
			}
			targetSize, err := intstr.GetScaledValueFromIntOrPercent(pp.policy.Spec.Policy.TargetSize, len(podList.Items), false)
			if err != nil {
				t.Fatalf("failed to get scaled value from int or percent: %v", err)
			}
			for _, p := range podList.Items {
				nodeName, err := getNodeName(cs, ns.Name, p.Name)
				if err != nil {
					t.Log(err)
				}
				if contains(pp.expectedNodes, nodeName) {
					targetCount++
				} else {
					t.Errorf("Pod %s is not assigned to the expected node(s)",
						p.Name)
				}
				if targetCount > targetSize {
					t.Errorf("The scheduler has exceeded the targetSize: %v",
						targetSize)
				}
			}
			t.Logf("case %v finished", pp.name)
		})
	}
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// getNodeName returns the name of the node if a node has assigned to the given pod
func getNodeName(c kubernetes.Interface, podNamespace, podName string) (string, error) {
	pod, err := c.CoreV1().Pods(podNamespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return pod.Spec.NodeName, nil
}

func makePlacementPolicyCRD() *apiextensionsv1.CustomResourceDefinition {
	content, err := os.ReadFile("../../config/crd/bases/placement-policy.scheduling.x-k8s.io_placementpolicies.yaml")
	if err != nil {
		klog.ErrorS(err, "Cannot read the yaml file")
		return &apiextensionsv1.CustomResourceDefinition{}
	}

	placementPoliciesCRD := &apiextensionsv1.CustomResourceDefinition{}
	err = yaml.Unmarshal(content, placementPoliciesCRD)
	if err != nil {
		klog.ErrorS(err, "Cannot parse the yaml file")
		return &apiextensionsv1.CustomResourceDefinition{}
	}

	return placementPoliciesCRD
}

func createPlacementPolicy(ctx context.Context, client versioned.Interface, placementpolicy *v1alpha1.PlacementPolicy) error {
	klog.Info("Creating placement policy")
	_, err := client.PlacementpolicyV1alpha1().PlacementPolicies(placementpolicy.Namespace).Create(ctx, placementpolicy, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func deletePlacementPolicy(ctx context.Context, client versioned.Interface, placementpolicy v1alpha1.PlacementPolicy) {
	client.PlacementpolicyV1alpha1().PlacementPolicies(placementpolicy.Namespace).Delete(ctx, placementpolicy.Name, metav1.DeleteOptions{})
}
