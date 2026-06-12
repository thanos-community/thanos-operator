# Feature Gates

The Thanos Operator provides several experimental features controlled by feature gates. These features are designed to extend the operator's capabilities while maintaining backward compatibility for the APIs. Features can be enabled individually using the `--enable-feature` command-line flag.

## Overview

Feature gates allow you to:
- Enable experimental functionality before it becomes stable
- Test new features in development environments
- Incrementally adopt new capabilities as they mature
- Maintain backward compatibility by keeping new features disabled by default

## Available Features

| Feature                                                 | Flag                 | Status       | Description                                                       |
|---------------------------------------------------------|----------------------|--------------|-------------------------------------------------------------------|
| [ServiceMonitor](#servicemonitor-feature)               | `service-monitor`    | Experimental | Automatic ServiceMonitor creation for Prometheus scraping         |
| [PrometheusRule](#prometheusrule-feature)               | `prometheus-rule`    | Experimental | Automatic discovery and mounting of PrometheusRule objects        |
| [OpenTelemetry Sidecar](#opentelemetry-sidecar-feature) | `otel-sidecar`       | Experimental | OpenTelemetry collector sidecar injection for distributed tracing |
| [KubeResourceSync](#kuberesourcesync-feature)           | `kube-resource-sync` | Experimental | Immediate ConfigMap/Secret synchronization via sidecar            |

## Enabling Features

### Command Line

Enable features using the `--enable-feature` flag when starting the operator:

```bash
# Enable a single feature
./thanos-operator --enable-feature service-monitor

# Enable multiple features
./thanos-operator \
  --enable-feature service-monitor \
  --enable-feature prometheus-rule \
  --enable-feature otel-sidecar
```

## ServiceMonitor Feature

**Flag**: `service-monitor`

### What It Achieves

Automatically creates and manages ServiceMonitor resources for Thanos components, eliminating the need to manually define ServiceMonitors for scraping the resources deployed by the operator. This feature integrates seamlessly with the Prometheus Operator ecosystem.

### How It Works

When enabled, the operator automatically creates ServiceMonitor resources alongside each Thanos component. These ServiceMonitors are configured with appropriate labels, selectors, and endpoints to enable Prometheus discovery and scraping.

### Prerequisites

- Prometheus Operator must be installed in the cluster
- Prometheus instance must be configured to discover ServiceMonitors with appropriate selectors

---

## PrometheusRule Feature

**Flag**: `prometheus-rule`

### What It Achieves

Enables ThanosRuler to automatically discover and mount PrometheusRule resources as configuration. This allows you to define alerting and recording rules as Kubernetes custom resources rather than manually managing ConfigMaps.

### How It Works

The operator watches for PrometheusRule resources that match the configured label selector, converts them into ConfigMaps, and mounts them into ThanosRuler pods. This provides automatic rule discovery and lifecycle management.

### Configuration

Configure PrometheusRule discovery in your ThanosRuler spec:

```yaml
apiVersion: monitoring.thanos.io/v1alpha1
kind: ThanosRuler
metadata:
  name: thanos-ruler
spec:
  ruleConfigSelector:
    matchLabels:
      prometheus: main
      role: alert-rules
  
  # Optional: Enable multi-tenancy
  ruleTenancyConfig:
    # Label on PrometheusRule that contains tenant value (default: "operator.thanos.io/tenant")
    tenantSpecifierLabel: tenant
    # Label injected into rule groups and PromQL expressions (default: "tenant_id") 
    enforcedTenantIdentifier: tenant_id
```

### PrometheusRule Example

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: thanos-alerts
  labels:
    prometheus: main
    role: alert-rules
    tenant: platform
spec:
  groups:
  - name: thanos.rules
    rules:
    - alert: ThanosQueryInstanceDown
      expr: up{job="thanos-query"} == 0
      for: 5m
      labels:
        severity: warning
      annotations:
        summary: "Thanos Query instance is down"
```

### PrometheusRule Discovery

The operator discovers PrometheusRule resources using the `ruleConfigSelector` you define. This is a standard Kubernetes label selector that determines which PrometheusRules the ThanosRuler should process.

**Discovery process**:
1. You define `ruleConfigSelector` with your desired labels
2. PrometheusRules matching the combined selector are discovered and processed

### Multi-Tenancy Processing

When `ruleTenancyConfig` is configured, the operator performs tenant injection:

1. **Tenant Discovery**:
   - Looks for `tenantSpecifierLabel` (default: `operator.thanos.io/tenant`) on PrometheusRule metadata
   - Uses the label value as the tenant identifier

2. **Rule Group Processing**:
   - Adds `enforcedTenantIdentifier` label to each rule group
   - Example: `tenant_id: platform` added to group labels

3. **PromQL Expression Enforcement**:
   - Injects tenant label into PromQL expressions using `enforceTenantLabelInPromQL`
   - Original: `up{job="thanos-query"} == 0`
   - Modified: `up{job="thanos-query", tenant_id="platform"} == 0`

**Tenancy Example**:

Input PrometheusRule:

```yaml
metadata:
  labels:
    tenant: platform  # tenantSpecifierLabel value
spec:
  groups:
  - name: thanos.rules
    rules:
    - alert: ThanosQueryInstanceDown
      expr: up{job="thanos-query"} == 0
```

Generated ConfigMap content:

```yaml
groups:
- name: thanos.rules
  labels:
    tenant_id: platform  # enforcedTenantIdentifier added
  rules:
  - alert: ThanosQueryInstanceDown
    expr: up{job="thanos-query", tenant_id="platform"} == 0  # PromQL modified
```

### Rule Processing

The operator continuously watches and processes PrometheusRule resources:

- **Discovery**: Uses the `ruleConfigSelector` to find matching PrometheusRules in the same namespace
- **Tenancy Processing**: When `ruleTenancyConfig` is enabled, applies tenant label injection to rule groups and PromQL expressions
- **Conversion**: Transforms PrometheusRule specs into Prometheus rule file format
- **ConfigMap Generation**: Creates ConfigMaps named `{ruler-name}-promrule-{index}` containing the rule files
- **Mounting**: ThanosRuler pods automatically mount these ConfigMaps as rule files
- **Precedence**: PrometheusRule-derived ConfigMaps override any conflicting user-created ConfigMaps
- **Metrics**: Exposes discovery and processing metrics including per-tenant rule counts

### Use Cases

- **GitOps rule management**: Store rules in version control as PrometheusRule resources
- **Multi-tenant alerting**: Separate rules per tenant with automatic label injection
- **Dynamic rule updates**: Rules update automatically when PrometheusRule resources change

---

## OpenTelemetry Sidecar Feature

**Flag**: `otel-sidecar`

### What It Achieves

Enables automatic injection of OpenTelemetry collector sidecars into Thanos component pods, providing distributed tracing capabilities across the entire Thanos stack without manual configuration.

### How It Works

When enabled, the operator adds the `sidecar.opentelemetry.io/inject: "true"` annotation to Thanos pods and configures Thanos components with OTLP tracing endpoints. The OpenTelemetry Operator handles the actual sidecar injection.

### Automatic Configuration

The operator automatically adds tracing configuration to Thanos components:

```yaml
# Automatically added tracing config
--tracing.config=type: OTLP
config:
  client_type: http
  endpoint: localhost:4318
  insecure: true
```

### Prerequisites

- OpenTelemetry Operator must be installed in the cluster
- [OpenTelemetryCollector resource must be configured for sidecar injection](https://opentelemetry.io/docs/kubernetes/operator/automatic/)

---

## KubeResourceSync Feature

**Flag**: `kube-resource-sync`

### What It Achieves

Provides immediate ConfigMap and Secret synchronization via a specialized sidecar container, eliminating kubelet sync delays (typically 60+ seconds) for critical configuration updates. Currently implemented for ThanosReceive hashring configuration.

### How It Works

The operator injects a kube-resource-sync sidecar container that watches Kubernetes resources in real-time and immediately syncs changes to a shared volume. An init container ensures data is available before the main Thanos container starts.

### Implementation Details

When enabled for ThanosReceive router:

1. **Volume Change**: ConfigMap volume mount is replaced with EmptyDir
2. **Init Container**: Ensures initial configuration is synced before Thanos starts
3. **Sidecar Container**: Continuously watches for configuration changes
4. **RBAC**: Automatically creates Role and RoleBinding for resource access

### RBAC Configuration

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: thanos-receive
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "watch"]
  resourceNames: ["thanos-receive"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: thanos-receive
subjects:
- kind: ServiceAccount
  name: thanos-receive
roleRef:
  kind: Role
  name: thanos-receive
  apiGroup: rbac.authorization.k8s.io
```

### Custom Image Configuration

```bash
# Environment variable
export KUBE_RESOURCE_SYNC_IMAGE=custom-registry/kube-resource-sync:v1.0.0
```

---

## Monitoring Feature Gates

The operator exposes metrics about enabled feature gates:

```promql
# Check which features are enabled
thanos_operator_feature_gates_info{feature="service-monitor"} == 1
```

## See Also

- [Prometheus Operator Documentation](https://prometheus-operator.dev/)
- [OpenTelemetry Documentation](https://opentelemetry.io/)
- [kube-resource-sync Project](https://github.com/philipgough/kube-resource-sync)
- [Thanos Documentation](https://thanos.io/)
