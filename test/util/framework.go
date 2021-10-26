package util

import (
	"k8s.io/kubernetes/pkg/scheduler/apis/config"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"k8s.io/kubernetes/pkg/scheduler/framework/runtime"
	st "k8s.io/kubernetes/pkg/scheduler/testing"
)

// NewFramework is a variant version of st.NewFramework - with extra PluginConfig slice as input.
func NewFramework(fns []st.RegisterPluginFunc, cfgs []config.PluginConfig, profileName string, opts ...runtime.Option) (framework.Framework, error) {
	registry := runtime.Registry{}
	plugins := &config.Plugins{}
	for _, f := range fns {
		f(&registry, plugins)
	}
	profile := &config.KubeSchedulerProfile{
		SchedulerName: profileName,
		Plugins:       plugins,
		PluginConfig:  cfgs,
	}
	return runtime.NewFramework(registry, profile, opts...)
}
