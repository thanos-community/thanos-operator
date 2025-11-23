---
weight: 30
toc: true
title: ThanosQuery
summary: Documentation for Thanos Operator components
slug: thanosquery.md
draft: false
description: Documentation for Thanos Operator components
---

The `ThanosQuery` CRD manages the Thanos Querier which provides a global PromQL query view across all types of Thanos data sources through the StoreAPI. It automatically discovers and connects to Store API endpoints and optionally can deploy a Query Frontend for improved query response times.

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

---

Found a typo, inconsistency or missing information in our docs? Help us to improve [Thanos Operator](https://thanos-operator.dev) documentation by proposing a fix [on GitHub here](https://github.com/thanos-community/thanos-operator/edit/main/docs/components/thanosquery.md) :heart:
