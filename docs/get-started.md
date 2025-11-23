# Get Started

Thanos Operator is a Kubernetes operator for managing [Thanos](https://thanos.io/) components. It provides custom resources to deploy and configure Thanos components in a cloud-native way.

## Prerequisites

Before getting started, ensure you have:

- A Kubernetes cluster (v1.19 or later)
- `kubectl` configured to access your cluster
- Basic understanding of [Thanos](https://thanos.io/) concepts

For detailed installation instructions, see the [Installation Guide](installation.md).

## Installation

### Install using YAML manifests

The easiest way to install Thanos Operator is using the provided bundle:

```bash
curl -sL https://raw.githubusercontent.com/thanos-community/thanos-operator/refs/heads/main/bundle.yaml | kubectl create -f -
```

This installs:
- Custom Resource Definitions (CRDs)
- RBAC resources
- The operator deployment in the `thanos-operator-system` namespace

### Custom namespace installation

To install in a different namespace using Kustomize:

```bash
NAMESPACE=my-namespace
TMPDIR=$(mktemp -d)
curl -sL https://raw.githubusercontent.com/thanos-community/thanos-operator/refs/heads/main/kustomization.yaml > "$TMPDIR/kustomization.yaml"
curl -sL https://raw.githubusercontent.com/thanos-community/thanos-operator/refs/heads/main/bundle.yaml > "$TMPDIR/bundle.yaml"
(cd $TMPDIR && kustomize edit set namespace $NAMESPACE) && kubectl create -k "$TMPDIR"
```

### Verify installation

Wait for the operator to be ready:

```bash
kubectl wait --for=condition=Ready pod \
  -l app.kubernetes.io/component=manager,app.kubernetes.io/part-of=thanos-operator,control-plane=controller-manager \
  -n thanos-operator-system \
  --timeout=2m
```

## Quick Start Example

Once the operator is installed, you can deploy a simple Thanos setup using the provided examples:

```bash
git clone https://github.com/thanos-community/thanos-operator.git
kubectl config use-context <YOUR-CLUSTER>
make install-example
```

This creates:
- MinIO object storage
- ThanosReceive for ingesting metrics
- ThanosQuery for querying metrics
- ThanosStore for long-term storage access
- ThanosCompact for data compaction

## Available Custom Resources

Thanos Operator provides the following CRDs:

- **ThanosReceive**: Manages Thanos Receive components for metrics ingestion
- **ThanosQuery**: Manages Thanos Query components for federated querying
- **ThanosStore**: Manages Thanos Store Gateway for object storage access
- **ThanosCompact**: Manages Thanos Compactor for data retention and downsampling
- **ThanosRuler**: Manages Thanos Ruler for alerting and recording rules

## Next Steps

- Explore the [component CRDs](components/thanosreceive.md) and how to use them.
- Explore the [API Reference](api-reference/api.md) for detailed CRD specifications
- Review [design decisions](proposals/design.md) for architectural insights
- Check out example configurations in the project's [`config/samples`](https://github.com/thanos-community/thanos-operator/tree/main/config/samples) directory
- Learn how to [contribute](../CONTRIBUTING.md) to the project
