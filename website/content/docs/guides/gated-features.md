# Feature Gates

Feature gates enable experimental Thanos Operator functionality. All features are disabled by default.

## Enabling and Configuring Features

Use `--enable-feature` for each feature you want to enable:

```bash
./thanos-operator \
  --enable-feature service-monitor \
  --enable-feature kube-resource-sync \
  --feature-gate-config-file /etc/thanos-operator/feature-gates.yaml
```

`--feature-gate-config-file` sets the YAML configuration path, which defaults to `/etc/thanos-operator/feature-gates.yaml`. The file configures features; it does not enable them. Omitted files, sections, or settings use defaults. Restart the operator after changing flags or configuration.

Only `service-monitor` and `kube-resource-sync` have config-file settings.

## ServiceMonitor Feature

**Flag**: `service-monitor`

Creates ServiceMonitors for Prometheus to scrape Thanos components. Requires Prometheus Operator and a Prometheus instance configured to discover the monitors.

```yaml
service-monitor:
  additionalLabels:
    prometheus: platform
  interval: 30s
```

These settings apply to all generated ServiceMonitors:

| Setting            | Default | Description                                                                                                                           |
|--------------------|---------|---------------------------------------------------------------------------------------------------------------------------------------|
| `additionalLabels` | `{}`    | Extra metadata labels for matching Prometheus's `spec.serviceMonitorSelector`. Existing workload and operator labels take precedence. |
| `interval`         | `""`    | Scrape interval. Empty uses Prometheus's global interval; otherwise, use a positive duration such as `30s` or `1m`.                   |

## PrometheusRule Feature

**Flag**: `prometheus-rule`

Lets ThanosRuler discover and use PrometheusRule resources for recording and alerting rules. Requires Prometheus Operator.

Configure discovery with `ThanosRuler.spec.ruleConfigSelector`. The default selector matches `operator.thanos.io/prometheus-rule: "true"`.

## OpenTelemetry Sidecar Feature

**Flag**: `otel-sidecar`

Enables OpenTelemetry collector sidecars for distributed tracing, with Thanos sending OTLP traces over HTTP to `localhost:4318`. Requires OpenTelemetry Operator and an [OpenTelemetryCollector configured for sidecar injection](https://opentelemetry.io/docs/kubernetes/operator/automatic/).

## KubeResourceSync Feature

**Flag**: `kube-resource-sync`

Adds a sidecar to ThanosReceive routers to synchronize hashring configuration changes immediately, avoiding kubelet ConfigMap update delays.

The `image` setting selects the container image. Its default is shown below:

```yaml
kube-resource-sync:
  image: quay.io/philipgough/kube-resource-sync:0.1.0
```

## Volume Resize Feature

**Flag**: `volume-resize`

Automatically expands persistent volumes when you increase storage sizes in Thanos custom resources. Requires a StorageClass with `allowVolumeExpansion: true`. Volumes can only grow, not shrink.
