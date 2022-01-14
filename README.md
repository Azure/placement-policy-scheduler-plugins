# Placement Policy Scheduler Plugins

A stand alone scheduler that wraps current scheduler and uses [scheduler framework](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/) with two additional plugins. The scheduler will run side by side with existing kubernetes scheduler and users will have to [specifically specify this scheduler](https://kubernetes.io/docs/tasks/extend-kubernetes/configure-multiple-schedulers/) to use for their workloads that perform the following:

- A scorer plugin implemented with that will be used in case “best effort” policy enforcement.
  - Extension points implemented: [PreScore](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/#pre-score) and [Score](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/#score)
- A filter plugin that will be used in case “force” policy enforcement.
  - Extension points implemented: [PreFilter](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/#pre-filter) and [Filter](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduling-framework/#filter)

**WARNING:** This is experimental code. It is not considered production-grade by its developers, nor is it "supported" software.

## Install

The container images for the scheduler plugin is available in the github container registry.

### Quick Install

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

### Helm

```bash
helm repo add placement-policy-scheduler-plugins https://azure.github.io/placement-policy-scheduler-plugins/charts
helm repo update

helm install -n kube-system [RELEASE_NAME] placement-policy-scheduler-plugins/placement-policy-scheduler-plugins
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
