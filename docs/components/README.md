# Components

Documentation for all Thanos Operator components and their Custom Resource Definitions (CRDs).

## Private image registries

Set `imagePullSecrets` on a component's spec to reference image pull Secrets in the same namespace as the workload. The operator copies these references to the generated Pod template; create and manage the Secrets separately.

| Component                    | Field                                            |
|------------------------------|--------------------------------------------------|
| Query, Ruler, Store, Compact | `spec.imagePullSecrets`                          |
| Query Frontend               | `spec.queryFrontend.imagePullSecrets`            |
| Receive Router               | `spec.routerSpec.imagePullSecrets`               |
| Receive Ingester             | `spec.ingesterSpec.hashrings[].imagePullSecrets` |

For example:

```yaml
apiVersion: monitoring.thanos.io/v1alpha1
kind: ThanosQuery
metadata:
  name: example
  namespace: monitoring
spec:
  imagePullSecrets:
    - name: private-registry
```

Query Frontend and each Receive router or ingester hashring use their own list. When the list is omitted or empty, Pods can inherit pull secrets from their ServiceAccount.

Changing or removing the list updates the Pod template and rolls out Pods according to the workload's update strategy. Upgrading the operator also applies lists already set on existing CRs, which can trigger a rollout.
