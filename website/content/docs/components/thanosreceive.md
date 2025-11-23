---
weight: 30
toc: true
title: ThanosReceive
summary: ThanosReceive manages metrics ingestion and routing in Thanos
seo:
    canonical: ""
    description: Complete documentation for ThanosReceive Custom Resource Definition
    robots: ""
    title: ThanosReceive CRD Reference
lastmod: "2024-01-15T09:00:00+00:00"
draft: false
description: Complete reference for ThanosReceive CRD
date: "2024-01-15T09:00:00+00:00"
---

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
                    │  (Load Balancer)    │
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
        storageSize: "100Mi"
        tsdbConfig:
          retention: 2h
        tenancyConfig:
          tenantMatcherType: exact
        replicas: 1
        externalLabels:
          replica: $(POD_NAME)
      - name: green
        storageSize: "100Mi"
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
