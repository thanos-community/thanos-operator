# ThanosReceive

The `ThanosReceive` CRD manages the ingestion of metrics via Prometheus remote-write protocol. It deploys and configures both router and ingester components to handle metric ingestion at scale.

## Overview

ThanosReceive consists of two main components:

- **Router**: Routes incoming remote-write requests to appropriate ingesters based on hashring configuration
- **Ingesters**: Store metrics in local TSDB and upload blocks to object storage

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Prometheus    │    │   Prometheus    │    │   Prometheus    │
│   Instance 1    │    │   Instance 2    │    │   Instance N    │
└─────────┬───────┘    └─────────┬───────┘    └─────────┬───────┘
          │                      │                      │
          └──────────────────────┼──────────────────────┘
                                 │ Remote Write
                                 ▼
                    ┌─────────────────────┐
                    │  ThanosReceive      │
                    │  Router             │
                    │  (route RW requests)│
                    └─────────┬───────────┘
                              │
              ┌───────────────┼───────────────┐
              │               │               │
              ▼               ▼               ▼
    ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
    │ ThanosReceive   │ │ ThanosReceive   │ │ ThanosReceive   │
    │ Ingester 1      │ │ Ingester 2      │ │ Ingester 3      │
    │ (Hashring A)    │ │ (Hashring A)    │ │ (Hashring B)    │
    └─────────┬───────┘ └─────────┬───────┘ └─────────┬───────┘
              │                   │                   │
              ▼                   ▼                   ▼
    ┌─────────────────────────────────────────────────────────┐
    │              Object Storage (S3, GCS, etc.)             │
    └─────────────────────────────────────────────────────────┘
```

## Basic Configuration

### Minimal Example

```yaml
apiVersion: monitoring.thanos.io/v1alpha1
kind: ThanosReceive
metadata:
  name: example-receive
spec:
  ingesterSpec:
    defaultObjectStorageConfig:
      name: thanos-object-storage
      key: thanos.yaml
    hashrings:
      - name: blue
        storage:
          size: "100Mi"
        tsdbConfig:
          retention: 2h
        tenancyConfig:
          tenantMatcherType: exact
        replicas: 1
        externalLabels:
          replica: $(POD_NAME)
      - name: green
        storage:
          size: "100Mi"
        tsdbConfig:
          retention: 2h
        tenancyConfig:
          tenantMatcherType: exact
        replicas: 1
        externalLabels:
          replica: $(POD_NAME)
  routerSpec:
    logFormat: logfmt
    imagePullPolicy: IfNotPresent
    externalLabels:
      receive: "true"
    replicas: 1
    replicationFactor: 1
```
