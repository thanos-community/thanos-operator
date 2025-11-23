# ThanosRuler

The `ThanosRuler` CRD manages the Thanos Ruler component, which evaluates Prometheus recording and alerting rules using data from Thanos Query. It provides distributed rule evaluation with high availability and multi-tenancy support.

## Overview

ThanosRuler performs several key functions:

- **Rule Evaluation**: Executes Prometheus recording and alerting rules at regular intervals
- **Alert Generation**: Sends alerts to Alertmanager based on rule evaluation results
- **Recording Rules**: Creates new time series from existing data
- **Multi-tenancy**: Supports tenant-specific rule isolation
- **High Availability**: Provides leader election and distributed evaluation

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ PrometheusRule  │    │ PrometheusRule  │    │   ConfigMap     │
│    (Tenant A)   │    │    (Tenant B)   │    │  (Rule Files)   │
└─────────┬───────┘    └─────────┬───────┘    └─────────┬───────┘
          │                      │                      │
          └──────────────────────┼──────────────────────┘
                                 │ Rule Discovery
                                 ▼
                    ┌─────────────────────┐
                    │    ThanosRuler      │
                    │   StatefulSet       │
                    │                     │
                    │ ┌─────────────────┐ │
                    │ │ Rule Evaluation │ │
                    │ │ Engine          │ │
                    │ └─────────────────┘ │
                    └─────────┬───────────┘
                              │
              ┌───────────────┼───────────────┐
              │               │               │
              ▼               ▼               ▼
    ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
    │  ThanosQuery    │ │  Alertmanager   │ │ Object Storage  │
    │  (Data Source)  │ │  (Alerts)       │ │ (Rule Results)  │
    └─────────────────┘ └─────────────────┘ └─────────────────┘
```

## Basic Configuration

### Minimal Example

```yaml
apiVersion: monitoring.thanos.io/v1alpha1
kind: ThanosRuler
metadata:
  name: example-ruler
spec:
  replicas: 1
  ruleConfigSelector:
    matchLabels:
      operator.thanos.io/rule-file: "true"
      operator.thanos.io/tenant: "thanos"
  queryLabelSelector:
    matchLabels:
      operator.thanos.io/query-api: "true"
      app.kubernetes.io/part-of: "thanos"
  defaultObjectStorageConfig:
    name: thanos-object-storage
    key: thanos.yaml
  alertmanagerURL: "http://alertmanager.example.com:9093"
  externalLabels:
    rule_replica: "$(NAME)"
  evaluationInterval: 1m
  retention: 2h
  storage:
    size: 1Gi
  logFormat: logfmt
  imagePullPolicy: IfNotPresent
```
