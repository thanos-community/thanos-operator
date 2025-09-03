---
title: "ThanosStore"
description: "Complete reference for ThanosStore CRD"
summary: "ThanosStore manages Store Gateway deployments for accessing historical data from object storage"
date: 2024-01-15T09:00:00+00:00
lastmod: 2024-01-15T09:00:00+00:00
draft: false
weight: 930
toc: true
seo:
  title: "ThanosStore CRD Reference"
  description: "Complete documentation for ThanosStore Custom Resource Definition"
  canonical: ""
  robots: ""
---

The `ThanosStore` CRD manages Store Gateway deployments that provide access to historical data stored in object storage. It supports automatic sharding, caching, and performance optimization for large-scale deployments.

## Overview

ThanosStore Gateway acts as a bridge between object storage and the Thanos query layer, providing:

- **Object Storage Access**: Reads historical blocks from object storage (S3, GCS, Azure, etc.)
- **Automatic Sharding**: Distributes blocks across multiple store instances for scalability
- **Caching**: Supports both index and bucket caching for improved performance
- **Time Range Filtering**: Serves only data within specified time ranges

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  ThanosQuery    │    │  ThanosQuery    │    │  ThanosQuery    │
│                 │    │                 │    │                 │
└─────────┬───────┘    └─────────┬───────┘    └─────────┬───────┘
          │                      │                      │
          └──────────────────────┼──────────────────────┘
                                 │ StoreAPI gRPC
                                 ▼
              ┌─────────────────────────────────────────────┐
              │           ThanosStore Shards                │
              │  ┌─────────┐ ┌─────────┐ ┌─────────┐       │
              │  │ Shard 0 │ │ Shard 1 │ │ Shard N │  ...  │
              │  │ (Hash   │ │ (Hash   │ │ (Hash   │       │
              │  │  mod 0) │ │  mod 1) │ │  mod N) │       │
              │  └─────────┘ └─────────┘ └─────────┘       │
              └─────────┬───────────────────────────────────┘
                        │
                        ▼
              ┌─────────────────────────────────────────────┐
              │            Object Storage                   │
              │  ┌─────────┐ ┌─────────┐ ┌─────────┐       │
              │  │ Block A │ │ Block B │ │ Block C │  ...  │
              │  │(Tenant1)│ │(Tenant2)│ │(Tenant1)│       │
              │  └─────────┘ └─────────┘ └─────────┘       │
              └─────────────────────────────────────────────┘
```

## Basic Configuration

### Minimal Example

```yaml
apiVersion: monitoring.thanos.io/v1alpha1
kind: ThanosStore
metadata:
  name: example-store
spec:
  imagePullPolicy: IfNotPresent
  logFormat: logfmt
  objectStorageConfig:
    name: thanos-object-storage
    key: thanos.yaml
  shardingStrategy:
    shards: 2
    type: block
  storageSize: 1Gi
  ignoreDeletionMarksDelay: 24h
  labels:
    some-label: xyz
```

