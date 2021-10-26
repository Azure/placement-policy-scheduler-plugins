package integration

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	st "k8s.io/kubernetes/pkg/scheduler/testing"
)

type resourceWrapper struct{ v1.ResourceList }

func MakeResourceList() *resourceWrapper {
	return &resourceWrapper{v1.ResourceList{}}
}

func (r *resourceWrapper) CPU(val int64) *resourceWrapper {
	r.ResourceList[v1.ResourceCPU] = *resource.NewQuantity(val, resource.DecimalSI)
	return r
}

func (r *resourceWrapper) Mem(val int64) *resourceWrapper {
	r.ResourceList[v1.ResourceMemory] = *resource.NewQuantity(val, resource.DecimalSI)
	return r
}

func (r *resourceWrapper) GPU(val int64) *resourceWrapper {
	r.ResourceList["nvidia.com/gpu"] = *resource.NewQuantity(val, resource.DecimalSI)
	return r
}

func (r *resourceWrapper) Obj() v1.ResourceList {
	return r.ResourceList
}

type podWrapper struct{ *v1.Pod }

func MakePod(namespace, name string) *podWrapper {
	pod := st.MakePod().Namespace(namespace).Name(name).Obj()

	return &podWrapper{pod}
}

func (p *podWrapper) Phase(phase v1.PodPhase) *podWrapper {
	p.Pod.Status.Phase = phase
	return p
}

func (p *podWrapper) Container(request v1.ResourceList) *podWrapper {
	p.Pod.Spec.Containers = append(p.Pod.Spec.Containers, v1.Container{
		Name:  fmt.Sprintf("con%d", len(p.Pod.Spec.Containers)),
		Image: "image",
		Resources: v1.ResourceRequirements{
			Requests: request,
		},
	})
	return p
}

func (p *podWrapper) InitContainerRequest(request v1.ResourceList) *podWrapper {
	p.Pod.Spec.InitContainers = append(p.Pod.Spec.InitContainers, v1.Container{
		Name:  fmt.Sprintf("con%d", len(p.Pod.Spec.Containers)),
		Image: "image",
		Resources: v1.ResourceRequirements{
			Requests: request,
		},
	})
	return p
}

func (p *podWrapper) Node(name string) *podWrapper {
	p.Pod.Spec.NodeName = name
	return p
}

func (p *podWrapper) Obj() *v1.Pod {
	return p.Pod
}
