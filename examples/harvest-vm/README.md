# Running Placement Policy Scheduler Plugins in Harvest VM cluster

## AKS demo

#### 1. Create an [AKS](https://docs.microsoft.com/en-us/azure/aks/) cluster with harvest vm

```sh
# Create a resource group in East US
az group create --name harvestvm --location eastus

# Create a basic single-node AKS cluster
az aks create \
    --resource-group harvestvm \
    --name harvestaks \
    --vm-set-type VirtualMachineScaleSets \
    --node-vm-size Harvest_E2s_v3
    --node-count 3 \
    --generate-ssh-keys \
    --load-balancer-sku standard
```

Run `az aks get-credentials` command to get access credentials for the cluster:

```
az aks get-credentials --resource-group harvestvm --name harvestaks
```

Add a second node pool with 3 nodes:

```
az aks nodepool add \
    --resource-group harvestvm \
    --cluster-name harvestaks \
    --name normalvms \
    --node-count 3
```

#### 2. Deploy placement-policy-scheduler-plugins as a secondary scheduler

The container images for the scheduler plugin is available in the github container registry.

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

#### 3. Choose node label

To identify the harvest vms, we can use the instance type node label: `node.kubernetes.io/instance-type: <vm size>`.  Run the following command to get the vm size:

 ```
 az aks nodepool list --cluster-name harvestvm -g harvest -o table
 ```

 <details>
 <summary>Result</summary>

 ```
 Name       OsType    KubernetesVersion    VmSize           Count    MaxPods    ProvisioningState    Mode
---------  --------  -------------------  ---------------  -------  ---------  -------------------  ------
agentpool  Linux     1.22.2               Harvest_E2s_v3   3        110        Succeeded            System
normalvms  Linux     1.22.2               Standard_D2s_v3  3        110        Succeeded            System

 ```
 </details><br/>

 The node label would be `node.kubernetes.io/instance-type: Harvest_E2s_v3`

#### 4. Deploy a `PlacementPolicy` CRD

```yaml
kind: PlacementPolicy
metadata:
  name: harvest-strict-must
spec:
  weight: 100
  enforcementMode: Strict
  podSelector:
    matchLabels:
      app: nginx
  nodeSelector:
    matchLabels:
      # instance type can be one of the following( Harvest_E2s_v3, Harvest_E4s_v3, Harvest_E8s_v3)
      node.kubernetes.io/instance-type: Harvest_E2s_v3
  policy:
    action: Must
    targetSize: 40%
```

<details>
<summary>Result</summary>

```
placementpolicy.placement-policy.scheduling.x-k8s.io/harvest-strict-must created
```
</details><br/>

>The same node selector `node.kubernetes.io/instance-type: Harvest_E2s_v3` is a node label for `agentpool` 

#### 5. Deploy a `ReplicaSet` that will create 10 replicas

```sh
kubectl apply -f https://raw.githubusercontent.com/Azure/placement-policy-scheduler-plugins/main/examples/demo_replicaset.yml
```

<details>
<summary>Result</summary>

```
replicaset.apps/nginx created
```
</details><br/>

#### 6. Get pods with matching labels

```sh
kubectl get po -o wide -l app=nginx --sort-by="{.spec.nodeName}" -o wide

NAME          READY   STATUS    RESTARTS   AGE   IP            NODE                                NOMINATED NODE   READINESS GATES
nginx-jgp5l   1/1     Running   0          56s   10.244.0.15   aks-agentpool-33997223-vmss000000   <none>           <none>
nginx-cdb9z   1/1     Running   0          56s   10.244.2.11   aks-agentpool-33997223-vmss000001   <none>           <none>
nginx-wpxj9   1/1     Running   0          56s   10.244.2.12   aks-agentpool-33997223-vmss000001   <none>           <none>
nginx-xc2cr   1/1     Running   0          56s   10.244.1.10   aks-agentpool-33997223-vmss000002   <none>           <none>
nginx-xvqbb   1/1     Running   0          56s   10.244.7.5    aks-normalvms-23099053-vmss000000   <none>           <none>
nginx-dmb4h   1/1     Running   0          56s   10.244.7.6    aks-normalvms-23099053-vmss000000   <none>           <none>
nginx-skzrk   1/1     Running   0          56s   10.244.8.6    aks-normalvms-23099053-vmss000001   <none>           <none>
nginx-hrznh   1/1     Running   0          56s   10.244.8.5    aks-normalvms-23099053-vmss000001   <none>           <none>
nginx-6c87l   1/1     Running   0          56s   10.244.6.6    aks-normalvms-23099053-vmss000002   <none>           <none>
nginx-f9mm2   1/1     Running   0          56s   10.244.6.5    aks-normalvms-23099053-vmss000002   <none>           <none>
```

We will find the nodes which carry the same node selector defined in the harvest-strict-must `PlacementPolicy` have been assigned 40% of the workload as defined with `targetSize`.

#### 7. Clean up

- Delete [AKS](https://docs.microsoft.com/en-us/azure/aks/) cluster

```bash
az group delete --name harvestvm --yes --no-wait
```