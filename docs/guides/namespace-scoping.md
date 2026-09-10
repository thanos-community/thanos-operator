# Namespace Scoping

By default, Thanos Operator watches all namespaces. Use `--watch-namespace` to restrict an instance to one namespace:

```bash
./thanos-operator --watch-namespace=monitoring
```

The setting applies to component reconciliation, service discovery, status updates, and optional controllers such as volume resizing. It takes effect at startup; restart the operator to change it. An empty value preserves cluster-wide behavior.

## Watch the operator's own namespace

Use the Downward API to pass the Deployment's namespace:

```yaml
env:
  - name: POD_NAMESPACE
    valueFrom:
      fieldRef:
        fieldPath: metadata.namespace
args:
  - --watch-namespace=$(POD_NAMESPACE)
```

Add these entries to the manager container's existing environment and arguments. The Go deployment builder provides the same configuration through `config.WithWatchOwnNamespace()`.

## Run multiple instances

Deploy each operator in the namespace it manages and give the instances disjoint scopes. Each instance can enable its own [feature gates](./gated-features.md). A cluster-wide instance would also reconcile those namespaces, so stop or rescope it before introducing scoped instances.

Leader-election leases remain in each operator's deployment namespace. Replicas in that namespace share a lease; operators deployed in separate namespaces elect their leaders independently.

Watch scoping does not change installed RBAC. For namespaced workload permissions, bind the generated manager ClusterRole to the operator ServiceAccount using a RoleBinding in the managed namespace. Keep the separate leader-election RoleBinding in the deployment namespace. Secure operator metrics also require the existing cluster-scoped token-review and subject-access-review permissions.

## End-to-end tests

`make e2e-setup` installs shared dependencies and loads the operator image. Each suite then creates its own operator, ServiceAccount, and bindings in its test namespace. Features are disabled unless the suite passes them explicitly to `suite.Setup`.

`make test-e2e` passes `E2E_IMG` to the suites. When running a suite directly after setup, pass the same image if you changed the default:

```bash
E2E_IMG=example.com/thanos-operator:v0.0.1 go test ./test/e2e/namespace -ginkgo.v
```

Use a dedicated test cluster. Setup rejects a running legacy test operator in `thanos-operator-system` because it would overlap with the per-suite operators.
