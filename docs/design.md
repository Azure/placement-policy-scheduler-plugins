# Placement Policy Scheduler Plugins

## Problem Statement

Cluster operators run clusters with various and mixed nodes’ capabilities. Some of these nodes may be ephemeral VMs such as cloud preemptible nodes such as those on [Azure](https://docs.microsoft.com/en-us/azure/batch/batch-low-pri-vms) or on [GCP](https://cloud.google.com/compute/docs/instances/preemptible). These nodes match the performance of standard nodes but the hosting provider does not provide strong guarantees on node availability or uptime due to the low cost nature of these VMs.

Cluster operators are typically left with the possible total loss of workloads running on such VMs if and when the hosting provider decides to preempt the entire node pool (or similar constructs) unless they ensure that workloads run on both preemptible and non-preemptible VMs which creates:
1. Additional scheduling complexities cluster wide and/or
2. Additional cogs that can and will offset cost saved by running on preemptible VMs in the first place.

## Example Use Cases

**Case 1**: As a cluster admin running a web facing workload I want to run a max of 40% of web serving deployment replicas on ephemeral nodes. That i am guaranteed 60% of my serving capacity up and running even when my hosting provider decides to preempt ephemeral nodes.

**Case 2**: As a cluster admin running a queue processing workload I want to ensure that a min of 80% runs on cheaper slower nodes. By utilizing older h/w i can save the overall cost of running my queue consumers. But if I don’t have enough capacity on older h/w I am perfectly fine running my queue consumers on the more expensive newer h/w.

## Goals

1. Create a policy based scheduling approach where cluster operators can specify percent of workload that can run on certain node types.
2. Enable best effort, and strict style policy based scheduling enforcement.

## Non-Goals

1. Create a full blown scheduler replacing the current stock scheduler.
2. Change any of the current kubernetes scheduling tool chain.

## Design & apis

### Overview

The proposed solution can be briefly described as the following:
- A new stand alone scheduler that wraps current scheduler and uses [scheduler framework](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/) with two additional plugins. The scheduler will run side by side with existing kubernetes scheduler and users will have to [specifically specify this scheduler](https://kubernetes.io/docs/tasks/extend-kubernetes/configure-multiple-schedulers/) to use for their workloads. that perform the following:
  - A scorer plugin (a scorer extension) that will be used in case “best effort” policy enforcement.
    - Extension points implemented: [PreScore](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/#pre-score) and [Score](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/#score)
  - A filter plugin that will be used in case “force” policy enforcement.
    - Extension points implemented: [PreFilter](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/#pre-filter) and [Filter](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/#filter) 
- A set of new apis based on Custom Resource Definition that drive the various expected behaviors.
- A validation and mutation web hooks that works on defaulting and validating the apis as needed.

### apis

A placement policy is an api object that specifies which nodes can host which pods. 

```yaml
apiVersion: "placement-policy.scheduling.x-k8s.io/v1"
kind: PlacementPolicy
metadata:
  name: <policy-name>
Spec:
  # The policy weight allows the engine to decide which policy to use when
  # pods match multiple policies. If multiple policies matched and all      
  # share the same weight then a policy with spec.enforcementMode == Force   
  # will be selected. If multiple policies match and +1 policy is marked 
  # as “Force” enforcementMode then they will sorted alphabetically / 
  # ascending and first one will be used. The scheduler publishes events 
  # capturing this conflict when it happens. Weight == 0-100 is reserved 
  # for future use.
  weight: <int> 
  # enforcementMode is an enum that specifies how the policy will be 
  # enforced during scheduler (e.g. the application of filter vs scorer 
  # plugin). Values allowed for this field are:
  # BestEffort (default): the policy will be enforced as best effort 
  # (scorer mode).
  # Strict: the policy will be forced during scheduling. The filter 
  # approach will be used. Note: that may yield pods unschedulable.
  enforcementMode: <string enum>
  # podSelector identifies which pods this placement policy will apply on
  podSelector:
    matchLabels:
      Key: Val # labels
  # nodeSelector selects the nodes where the placement policy will 
  # apply on according to action
  nodeSelector:
    matchLabels: # key/value selector
  placementPolicy:
  # The action field is policy placement action. It is a string enum 
  # that carries the following possible values:
  # Must(default): based on the rule below pods must be placed on 
  # nodes selected by node selector
  # MustNot: based on the rule pods must *not* be placed nodes selected by node selector
    action: <string enum>
    # absolute count or % of total.
    targetSize: xx% or xx
```
>Note: Scheduler is a stand alone binary similar to [this example](https://github.com/kubernetes-sigs/scheduler-plugins/blob/master/cmd/scheduler/main.go#L40). The scheduler performs filter or score based on PlacementPolicy.Spec.EnforcementMode value. 

#### Validation and Defaulting:

1. Node Selector must not have an empty label selector and an empty annotation selector.
2. Spec.Weight > 100

>Note: due to the way scheduler framework the system must provide a scheduler for each release similar to [this example](https://github.com/kubernetes-sigs/scheduler-plugins#compatibility-matrix). Initially implementation will be focused on kubernetes v.Current and v.Current - 1 (major only). A major scheduler release per a major kubernetes release should suffice unless Kubernetes upstream majorly patches its own scheduler (which is wrapped in our custom scheduler).