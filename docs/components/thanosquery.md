# ThanosQuery

The `ThanosQuery` CRD provides a global view across all Thanos data sources through the StoreAPI. It automatically discovers and connects to Store API endpoints and optionally deploys a Query Frontend for improved performance.

## Overview

ThanosQuery consists of two main components:

- **Query**: The core querying engine that federates data from multiple StoreAPI endpoints
- **Query Frontend** (Optional): Provides query caching, splitting, and retry logic

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│     Grafana     │    │     Perses      │    │  Other Clients  │
│                 │    │                 │    │                 │
└─────────┬───────┘    └─────────┬───────┘    └─────────┬───────┘
          │                      │                      │
          └──────────────────────┼──────────────────────┘
                                 │ PromQL Queries
                                 ▼
                    ┌─────────────────────┐
                    │  Thanos Query       │
                    │  Frontend           │
                    │  (Optional)         │
                    └─────────┬───────────┘
                              │
                              ▼
                    ┌─────────────────────┐
                    │  Thanos Query       │
                    │                     │
                    └─────────┬───────────┘
                              │
              ┌───────────────┼───────────────┐
              │               │               │
              ▼               ▼               ▼
    ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
    │ ThanosStore     │ │ ThanosReceive   │ │ Prometheus      │
    │ (Historical)    │ │ (Recent)        │ │ Sidecar         │
    └─────────────────┘ └─────────────────┘ └─────────────────┘
```

## Basic Configuration

### Minimal Example

```yaml
apiVersion: monitoring.thanos.io/v1alpha1
kind: ThanosQuery
metadata:
  name: example-query
spec:
  customStoreLabelSelector:
    matchLabels:
      operator.thanos.io/store-api: "true"
  imagePullPolicy: IfNotPresent
  labels:
    some-label: xyz
  logFormat: logfmt
  replicaLabels:
    - prometheus_replica
    - replica
    - rule_replica
  queryFrontend:
    compressResponses: true
    imagePullPolicy: IfNotPresent
    labelsMaxRetries: 3
    logFormat: logfmt
    logQueriesLongerThan: 10s
    queryLabelSelector:
      matchLabels:
        operator.thanos.io/query-api: "true"
    queryRangeMaxRetries: 3
    replicas: 2
  replicas: 1
```
