---
title: "Ecosystem Compatibility"
description: "How Thanos Operator integrates with existing Prometheus Operator deployments and the broader monitoring ecosystem"
summary: "Learn how Thanos Operator works alongside Prometheus Operator and other monitoring tools without conflicts"
date: 2025-01-29T16:00:00+00:00
lastmod: 2025-01-29T16:00:00+00:00
draft: false
weight: 850
toc: true
seo:
  title: "Thanos Operator Ecosystem Compatibility"
  description: "Comprehensive guide on how Thanos Operator integrates with Prometheus Operator and existing monitoring infrastructure"
  canonical: ""
  robots: ""
---

## Overview

Thanos Operator is designed to work seamlessly alongside existing monitoring infrastructure, particularly with upstream Prometheus Operator deployments. This guide explains how the operator integrates with your current ecosystem without causing conflicts or requiring extensive reconfiguration.

## Prometheus Operator Integration

### Native Compatibility

Thanos Operator is built with first-class support for Prometheus Operator environments:

- **CRD Coexistence**: Thanos Operator CRDs are designed to complement, not replace, Prometheus Operator CRDs
- **ServiceMonitor Discovery**: Automatic discovery of existing ServiceMonitor resources for query federation
- **AlertManager Integration**: Works with existing AlertManager instances managed by Prometheus Operator
- **PrometheusRule Support**: Leverages existing PrometheusRule CRDs for ThanosRuler deployments

### Feature Gates for Flexibility

The operator includes feature gates to control Prometheus Operator integration:

```yaml
apiVersion: monitoring.thanos.io/v1alpha1
kind: ThanosQuery
metadata:
  name: thanos-query
spec:
  featureGates:
    enablePrometheusOperatorCRDs: true
```

You can also control this at the operator level:

```bash
./manager -feature-gate.enable-prometheus-operator-crds=true
```

This design allows you to:
- Run Thanos Operator in environments **without** Prometheus Operator
- Gradually adopt Thanos components alongside existing Prometheus deployments
- Maintain backward compatibility with existing monitoring setups

## Deployment Scenarios

### Scenario 1: Existing Prometheus Operator Deployment

If you already have Prometheus Operator managing your Prometheus instances:

1. **Install Thanos Operator** alongside your existing setup
2. **Configure ThanosQuery** to discover existing Prometheus instances via ServiceMonitors
3. **Add ThanosStore** components for long-term storage access
4. **Optionally deploy ThanosReceive** for remote write capabilities

```yaml
apiVersion: monitoring.thanos.io/v1alpha1
kind: ThanosQuery
metadata:
  name: federated-query
spec:
  serviceMonitorSelector:
    matchLabels:
      prometheus: kube-prometheus
  featureGates:
    enablePrometheusOperatorCRDs: true
```

### Scenario 2: Fresh Installation

For new deployments, you can install both operators together:

1. **Install Prometheus Operator** for Prometheus and AlertManager management
2. **Install Thanos Operator** for Thanos component management
3. **Configure federation** between components using both operators

### Scenario 3: Thanos-Only Deployment

Thanos Operator can also run independently:

```yaml
apiVersion: monitoring.thanos.io/v1alpha1
kind: ThanosReceive
metadata:
  name: standalone-receive
spec:
  featureGates:
    enablePrometheusOperatorCRDs: false
```

## Component Integration Patterns

### ThanosQuery with Prometheus Discovery

ThanosQuery automatically discovers Prometheus instances managed by Prometheus Operator:

```yaml
apiVersion: monitoring.thanos.io/v1alpha1
kind: ThanosQuery
metadata:
  name: query-with-prometheus
spec:
  serviceDiscovery:
    prometheusServiceMonitors:
      enabled: true
      selector:
        matchLabels:
          app.kubernetes.io/name: prometheus
```

### ThanosRuler with PrometheusRule

ThanosRuler can leverage existing PrometheusRule resources:

```yaml
apiVersion: monitoring.thanos.io/v1alpha1
kind: ThanosRuler
metadata:
  name: ruler-with-prometheus-rules
spec:
  ruleSelector:
    matchLabels:
      prometheus: kube-prometheus
      role: alert-rules
```

### ServiceMonitor Generation

Thanos Operator automatically creates ServiceMonitor resources when Prometheus Operator CRDs are available, enabling seamless monitoring of Thanos components.

## Best Practices

### Namespace Organization

- **Separate namespaces**: Deploy Thanos Operator in a dedicated namespace (e.g., `thanos-system`)
- **Shared monitoring**: Use a common namespace for shared monitoring resources
- **RBAC isolation**: Maintain proper RBAC boundaries between operators

### Resource Naming

- **Consistent prefixes**: Use consistent naming conventions (e.g., `thanos-` prefix)
- **Label selectors**: Use specific label selectors to avoid resource conflicts
- **Annotation coordination**: Coordinate annotations between operators for shared resources

### Monitoring Integration

```yaml
# Example ServiceMonitor that works with both operators
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: thanos-components
  labels:
    app.kubernetes.io/component: monitoring
    app.kubernetes.io/part-of: thanos
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: thanos
```

## Migration Strategies

### Gradual Migration

1. **Install Thanos Operator** alongside existing Prometheus Operator
2. **Start with ThanosQuery** to federate existing Prometheus instances
3. **Add long-term storage** with ThanosStore components
4. **Implement remote write** using ThanosReceive
5. **Migrate alerting** to ThanosRuler gradually

### Validation Steps

Before migration:
- Verify CRD compatibility using `kubectl api-resources`
- Test operator permissions with dry-run deployments
- Validate service discovery configuration
- Check network policies and security contexts

## Troubleshooting

### Common Integration Issues

**CRD Conflicts**: Ensure both operators are using compatible CRD versions
```bash
kubectl get crd | grep -E "(prometheus|thanos)"
```

**RBAC Permissions**: Verify cross-operator permissions for shared resources
```bash
kubectl auth can-i get servicemonitors --as=system:serviceaccount:thanos-system:thanos-operator
```

**Service Discovery**: Debug service discovery configuration
```bash
kubectl logs -n thanos-system deployment/thanos-operator-controller-manager
```

### Feature Gate Debugging

Check if Prometheus Operator CRDs are properly detected:

```bash
kubectl logs -n thanos-system deployment/thanos-operator-controller-manager | grep "prometheus-operator"
```

## Ecosystem Benefits

### Unified Monitoring

- **Single pane of glass**: Query across all metrics through ThanosQuery
- **Consistent alerting**: Unified alerting rules across all environments
- **Cost optimization**: Efficient long-term storage with ThanosStore

### Operational Advantages

- **Independent scaling**: Scale Prometheus and Thanos components independently
- **Zero-downtime upgrades**: Upgrade operators independently
- **Flexible deployment**: Choose components based on specific needs

The Thanos Operator's ecosystem-first design ensures that adopting Thanos doesn't disrupt your existing monitoring infrastructure while providing a clear path to enhanced observability capabilities.