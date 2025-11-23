# ThanosCompact

The `ThanosCompact` CRD manages the Thanos Compactor component, which handles data compaction, downsampling, and retention management for object storage blocks. It's essential for maintaining optimal storage efficiency and query performance over time.

## Overview

ThanosCompact performs several critical functions:

- **Block Compaction**: Combines smaller blocks into larger ones to reduce metadata overhead
- **Downsampling**: Creates lower-resolution data for faster long-term queries
- **Retention Management**: Removes old data according to configured retention policies
- **Block Cleanup**: Handles deletion of corrupted or marked blocks
- **Sharding**: Supports sharding for large-scale deployments

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Object Storage                               │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐        │
│  │ Block A │ │ Block B │ │ Block C │ │ Block D │ │ Block E │  ...   │
│  │(raw res)│ │(raw res)│ │(raw res)│ │(raw res)│ │(raw res)│        │
│  │ Tenant1 │ │ Tenant1 │ │ Tenant2 │ │ Tenant2 │ │ Tenant1 │        │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘        │
└─────────────────┬───────────────────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    ThanosCompact Shards                             │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐                    │
│  │   Shard 1   │ │   Shard 2   │ │   Shard 3   │                    │
│  │  Tenant1    │ │  Tenant2    │ │  Tenant3    │                    │
│  │             │ │             │ │             │                    │
│  │ ┌─────────┐ │ │ ┌─────────┐ │ │ ┌─────────┐ │                    │
│  │ │Compact  │ │ │ │Compact  │ │ │ │Compact  │ │                    │
│  │ │Downsamp.│ │ │ │Downsamp.│ │ │ │Downsamp.│ │                    │
│  │ │Retention│ │ │ │Retention│ │ │ │Retention│ │                    │
│  │ └─────────┘ │ │ └─────────┘ │ │ └─────────┘ │                    │
│  └─────────────┘ └─────────────┘ └─────────────┘                    │
└─────────────────┬───────────────────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    Optimized Object Storage                         │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐                    │
│  │ Compacted   │ │ Downsampled │ │ Retention   │                    │
│  │ Blocks      │ │ Blocks      │ │ Applied     │                    │
│  │ (Larger)    │ │ (5m, 1h res)│ │ (Old data   │                    │
│  │             │ │             │ │  removed)   │                    │
│  └─────────────┘ └─────────────┘ └─────────────┘                    │
└─────────────────────────────────────────────────────────────────────┘
```

## Basic Configuration

### Minimal Example

```yaml
apiVersion: monitoring.thanos.io/v1alpha1
kind: ThanosCompact
metadata:
  name: example-compact
spec:
  storage:
    size: "100Mi"
  shardingConfig:
    - shardName: example
      externalLabelSharding:
        - label: tenant_id
          value: "a"
  objectStorageConfig:
    name: thanos-object-storage
    key: thanos.yaml
  retentionConfig:
    raw: 30d
    fiveMinutes: 30d
    oneHour: 30d
```
