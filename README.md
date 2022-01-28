# Placement Policy Scheduler Plugins

[![Go Report Card](https://goreportcard.com/badge/github.com/Azure/placement-policy-scheduler-plugins)](https://goreportcard.com/report/github.com/Azure/placement-policy-scheduler-plugins)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/Azure/placement-policy-scheduler-plugins)

Most of cloud environments today provides cluster admins with ephemeral nodes (VMs). These nodes typically cost significantly less but they offer less reliability than their regular counterpart. Cluster admins are often torn between the choice of cost and reliability because of the innate inability of the default Kubernetes scheduler to place some of a specific workload pods on these nodes. Having the entire workload on ephemeral nodes risks the reliability of the workload when the cloud environment stops these nodes.

This scheduler enables cluster admins to offload some configurable percentage of their workloads on these nodes enabling them to decrease the cost of running these pods without affecting its reliability. The scheduler is implemented using the [scheduler framework](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/) with two additional plugins. The scheduler will run side by side with existing kubernetes scheduler and users will have to [specifically specify this scheduler](https://kubernetes.io/docs/tasks/extend-kubernetes/configure-multiple-schedulers/) to use for their workloads that perform the following:

- A scorer plugin implemented with that will be used in case “best effort” policy enforcement.
  - Extension points implemented: [PreScore](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/#pre-score) and [Score](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/#score)
- A filter plugin that will be used in case “force” policy enforcement.
  - Extension points implemented: [PreFilter](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/#pre-filter) and [Filter](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/#filter)

**NOTE:** This code is in **ALPHA** status. It is not considered ready for production use.

## Quick Start

### Install

The container images for the scheduler plugin is available in the github container registry.

#### Quick Install

```bash
kubectl apply -f https://raw.githubusercontent.com/Azure/placement-policy-scheduler-plugins/main/deploy/kube-scheduler-configuration.yml
```

<details>
<summary>Result</summary>

```
customresourcedefinition.apiextensions.k8s.io/placementpolicies.placement-policy.scheduling.x-k8s.io created
configmap/pp-scheduler-config created
clusterrole.rbac.authorization.k8s.io/pp-plugins-scheduler created
clusterrolebinding.rbac.authorization.k8s.io/pp-plugins-scheduler created
rolebinding.rbac.authorization.k8s.io/pp-plugins-scheduler-as-kube-scheduler created
clusterrolebinding.rbac.authorization.k8s.io/pp-plugins-scheduler-as-kube-scheduler created
serviceaccount/pp-plugins-scheduler created
deployment.apps/pp-plugins-scheduler created

```
</details><br/>

#### Helm

```bash
helm repo add placement-policy-scheduler-plugins https://azure.github.io/placement-policy-scheduler-plugins/charts
helm repo update

helm install -n kube-system [RELEASE_NAME] placement-policy-scheduler-plugins/placement-policy-scheduler-plugins
```

### Example config

```yaml
apiVersion: placement-policy.scheduling.x-k8s.io/v1alpha1
kind: PlacementPolicy
metadata:
  name: besteffort-must
spec:
  weight: 100
  enforcementMode: BestEffort
  podSelector:
    matchLabels:
      app: nginx
  nodeSelector:
    matchLabels:
      node: want
  policy:
    action: Must
    targetSize: 40%
```

- **enforcementMode**: specifies how the policy will be enforced during scheduler. Values allowed for this field are:
  - **BestEffort** (default): the policy will be enforced as best effort (scorer mode).
  - **Strict**: the policy will be forced during scheduling.
- **nodeSelector**: selects the nodes where the placement policy will apply on according to action.
- **podSelector**: identifies which pods this placement policy will apply on
- **action**: policy placement action that carries the following possible values:
  - **Must**(default): based on the rule below pods must be placed on nodes selected by node selector MustNot: based on the rule pods
  - **MustNot** be placed nodes selected by node selector'
- **targetSize**: the number or percent of pods that can or cannot be placed on the node.
- **weight**: allows the engine to decide which policy to use when pods match multiple policies.

### Demo

#### 1. Create a [kind](https://kind.sigs.k8s.io/) cluster with the following config

```sh
cat <<EOF | kind create cluster --name placement-policy --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
  kubeadmConfigPatches:
    - |
      kind: JoinConfiguration
      nodeRegistration:
        kubeletExtraArgs:
          node-labels: "node=want"
- role: worker
  kubeadmConfigPatches:
    - |
      kind: JoinConfiguration
      nodeRegistration:
        kubeletExtraArgs:
          node-labels: "node=want"
- role: worker
  kubeadmConfigPatches:
    - |
      kind: JoinConfiguration
      nodeRegistration:
        kubeletExtraArgs:
          node-labels: "node=unwant"
EOF
```

<details>
<summary>Output</summary>

```bash
Creating cluster "placement-policy" ...
 ✓ Ensuring node image (kindest/node:v1.21.1) 🖼
 ✓ Preparing nodes 📦 📦 📦 📦
 ✓ Writing configuration 📜
 ✓ Starting control-plane 🕹️
 ✓ Installing CNI 🔌
 ✓ Installing StorageClass 💾
 ✓ Joining worker nodes 🚜
Set kubectl context to "kind-placement-policy"
You can now use your cluster with:

kubectl cluster-info --context kind-placement-policy

Have a question, bug, or feature request? Let us know! https://kind.sigs.k8s.io/#community 🙂
```
</details><br/>

>The same node selector `node: want` will be used as node label for `kind-worker` and `kind-worker2`

<details>
<summary>Node labels</summary>

```bash
➜ kubectl get nodes -o=custom-columns="NAME:.metadata.name,LABEL:.metadata.labels['node']"
NAME                 LABEL
kind-control-plane   <none>
kind-worker          want
kind-worker2         want
kind-worker3         unwant
```

</details><br/>

#### 2. Follow the [Install](#install) section to deploy placement-policy-scheduler-plugins as a secondary scheduler

#### 3. Deploy a `PlacementPolicy` CRD
  
```sh
kubectl apply -f https://raw.githubusercontent.com/Azure/placement-policy-scheduler-plugins/main/examples/v1alpha1_placementpolicy_strict_must.yml
```
<details>
<summary>Result</summary>

```
placementpolicy.placement-policy.scheduling.x-k8s.io/strict-must created
```
</details><br/>

#### 4. Deploy a `ReplicaSet` that will create 10 replicas

```sh
kubectl apply -f https://raw.githubusercontent.com/Azure/placement-policy-scheduler-plugins/main/examples/demo_replicaset.yml
```

<details>
<summary>Result</summary>

```
replicaset.apps/nginx created
```
</details><br/>

#### 5. Get pods with matching labels

```sh
kubectl get po -o wide -l app=nginx

NAME          READY   STATUS    RESTARTS   AGE   IP           NODE           NOMINATED NODE   READINESS GATES
nginx-8cr58   1/1     Running   0          76s   10.244.3.4   kind-worker3   <none>           <none>
nginx-d7js5   1/1     Running   0          76s   10.244.1.2   kind-worker2   <none>           <none>
nginx-jt527   1/1     Running   0          76s   10.244.3.6   kind-worker3   <none>           <none>
nginx-m5c86   1/1     Running   0          76s   10.244.2.2   kind-worker    <none>           <none>
nginx-qxx6m   1/1     Running   0          76s   10.244.3.2   kind-worker3   <none>           <none>
nginx-rdlzx   1/1     Running   0          76s   10.244.2.3   kind-worker    <none>           <none>
nginx-skk5z   1/1     Running   0          76s   10.244.1.3   kind-worker2   <none>           <none>
nginx-vq598   1/1     Running   0          76s   10.244.3.7   kind-worker3   <none>           <none>
nginx-xzxsb   1/1     Running   0          76s   10.244.3.3   kind-worker3   <none>           <none>
nginx-zwrsk   1/1     Running   0          76s   10.244.3.5   kind-worker3   <none>           <none>
```

We will find the nodes which carry the same node selector defined in the strict-must `PlacementPolicy` have been assigned 40% of the workload as defined with `targetSize`.

#### 6. Clean up

- Delete [kind](https://kind.sigs.k8s.io/) cluster

```bash
kind delete cluster --name placement-policy
```

## Contributing

This project welcomes contributions and suggestions.  Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit https://cla.opensource.microsoft.com.

When you submit a pull request, a CLA bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., status check, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.
