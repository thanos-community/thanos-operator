# SOP: Resizing Receive Hashring Volume

## Overview

This document provides a standard operating procedure for resizing a Thanos Receive hashring volume size.

## Problem Statement

The Thanos Receive component uses a hashring to distribute incoming data across multiple replicas. If the volume size allocated for the hashring is insufficient, it can lead to performance degradation. Resizing the hashring volume is necessary to ensure optimal performance and reliability of the Thanos Receive component.

Unfortunately, resizing the hashring volume is not a straightforward process. The volumes are created via a `volumeClaimTemplate` in the StatefulSet, and resizing them is not supported by Kubernetes.

See the following:
1. https://github.com/kubernetes/kubernetes/issues/68737
2. https://github.com/kubernetes/enhancements/issues/4650
3. https://github.com/kubernetes/enhancements/pull/4651

The progress of this KEP is being followed closely by the authors of this project, and we will add native support for resizing the hashring volume if/when it becomes available in Kubernetes.

There are various guides online that allow you to resize a PVC based on recreating the StatefulSet. This SOP provides an alternative approach that does not require recreating the StatefulSet and thus avoids downtime and data loss.

## Prerequisites

- Access to the Kubernetes cluster

## Procedure

### Step 1: Preparation

1. Identify the Thanos Receive Hashring volume that needs to be resized.
2. Determine the new desired size for the volume.
3. Identify the tenants that this hashring is routing for and ensure that they are aware of the maintenance window.

### Step 2: Create New Hashring with Larger Volume

The operator manages the hashrings by name starting alphabetically. The first step is to create a hashring with a name that will appear before the existing hashring in alphabetical order. This will ensure that the new hashring is matched and used for routing before the existing one. We recommend something like `active` or `a-{hashring-name}`. We will use `active` in this example.

Taking the example from the [ThanosReceive documentation](../../../components/thanosreceive.md/), modify your ThanosReceive resource with the new ingest configuration as follows:

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
      - name: active  # New hashring with larger storage
        storage:
          size: "500Mi"  # Increased from 100Mi
        tsdbConfig:
          retention: 2h
        tenancyConfig:
          tenantMatcherType: exact
        replicas: 1
        externalLabels:
          replica: $(POD_NAME)
      - name: blue  # Existing hashring to be replaced
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

### Step 3: Verification

After confirming that the new StatefulSet has all replicas available, it is essential to verify the traffic has drained from the old hashring and is now being routed to the new hashring.

This can be done by graphing the metric `sum by (service)(rate(prometheus_tsdb_head_series_created_total{}[2m]))`. This metric will show the number of series being created in each hashring.

### Step 4: Remove Old Hashring

Once the traffic has fully drained from the old hashring, you can safely remove it from the ThanosReceive resource.

### Step 5: Verify Resource Removal

If you are using the default `Delete` policy for the `persistentVolumeClaimRetentionPolicy` you should confirm that the PersistentVolumes and PersistentVolumeClaims associated with the old hashring are deleted before proceeding to the next step.

If you are using the `Retain` policy, you can either delete the resources manually or resize the PVC to the new desired size and reuse it for the new hashring assuming your StorageClass supports volume expansion.

See the following documentation for more details on how to resize a PersistentVolumeClaim:
* https://kubernetes.io/docs/concepts/storage/persistent-volumes/#resizing-persistent-volumes-claims

### Step 6: Reinstate Resized Hashring

Assuming you want to keep the same hashring name, you can now create a new hashring with the original name and the new desired volume size.

Once again, wait for the hashring to become fully ready and present in the hashring configuration before proceeding.

### Step 7: Remove Temporary Hashring

Finally, once the resized hashring is fully ready and receiving traffic, you can remove the temporary hashring that was created in Step 2.

### Step 8: Conclusion

Validate the change by ensuring the new hashring is functioning correctly and that there are no performance issues. The new resized hashring should now be fully operational and handling the traffic as expected for the tenants it is routing for.

Repeat Step 5 `Verify Resource Removal` for the temporary hashring.
