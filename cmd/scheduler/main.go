package main

import (
	"os"

	"github.com/Azure/placement-policy-scheduler-plugins/pkg/plugins/placementpolicy"

	"k8s.io/klog/v2"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"
	// +kubebuilder:scaffold:imports
)

func main() {
	// register the custom plugins with kube-scheduler
	command := app.NewSchedulerCommand(
		app.WithPlugin(placementpolicy.Name, placementpolicy.New))

	if err := command.Execute(); err != nil {
		klog.ErrorS(err, "unable to run command")
		os.Exit(1)
	}
}
