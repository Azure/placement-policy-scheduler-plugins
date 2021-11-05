//go:build e2e
// +build e2e

package node

import (
	"context"
	"fmt"

	"github.com/Azure/placement-policy-scheduler-plugins/test/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	schedtest "k8s.io/kubernetes/pkg/scheduler/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateInput is the input for Create.
type CreateInput struct {
	Creator framework.Creator
	Name    string
	Labels  map[string]string
}

type keyValue struct {
	k string
	v string
}

// Create creates a node.
func Create(input CreateInput) *corev1.Node {
	Expect(input.Creator).NotTo(BeNil(), "input.Creator is required for Node.Create")
	Expect(input.Name).NotTo(BeEmpty(), "input.Name is required for Node.Create")
	Expect(input.Labels).NotTo(BeEmpty(), "input.Labels is required for Node.Create")

	var labelPairs []keyValue
	for k, v := range input.Labels {
		labelPairs = append(labelPairs, keyValue{k: k, v: v})
	}

	nodeWrapper := schedtest.MakeNode().Name(input.Name)
	// apply labels[0], labels[0,1], ..., labels[all] to each pod in turn
	for _, p := range labelPairs[:len(labelPairs)+1] {
		nodeWrapper = nodeWrapper.Label(p.k, p.v)
	}

	node := nodeWrapper.Obj()

	Expect(input.Creator.Create(context.TODO(), node)).Should(Succeed())
	By(fmt.Sprintf("Creating node \"%s\"", node.Name))

	return node
}

// DeleteInput is the input for Delete.
type DeleteInput struct {
	Deleter framework.Deleter
	Getter  framework.Getter
	Node    *corev1.Node
}

// Delete deletes a node.
func Delete(input DeleteInput) {
	Expect(input.Deleter).NotTo(BeNil(), "input.Deleter is required for Node.Delete")
	Expect(input.Getter).NotTo(BeNil(), "input.Getter is required for Node.Delete")
	Expect(input.Node).NotTo(BeNil(), "input.Node is required for Node.Delete")

	By(fmt.Sprintf("Deleting node \"%s\"", input.Node.Name))
	Expect(input.Deleter.Delete(context.TODO(), input.Node)).Should(Succeed())

	Eventually(func() bool {
		return input.Getter.Get(context.TODO(), client.ObjectKey{Name: input.Node.Name}, &corev1.Node{}) != nil
	})
}
