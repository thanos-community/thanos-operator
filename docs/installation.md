# Installation

Currently, we offer a single mode of installing Thanos Operator, using a YAML bundle. Alternative methods will be added in the future!

## Pre-requisites

For all the approaches listed on this page, you require access to a Kubernetes cluster! For this, you can check the official docs of Kubernetes available [here](https://kubernetes.io/docs/tasks/tools/).

## Install using YAML files

The first step is to install the operator's Custom Resource Definitions (CRDs) as well as the operator itself with the required RBAC resources.

We've packaged all of them into a neat bundle for you! Run the command below to fetch and apply that bundle. This will install the CRDs, apply RBAC and deploy the operator binary in the `thanos-operator-system` namespace.

```bash
curl -sL https://raw.githubusercontent.com/thanos-community/thanos-operator/refs/heads/main/bundle.yaml | kubectl create -f -
```

If you would like to install the operator in a different namespace, you can use [Kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/) to do so,

```bash
NAMESPACE=my_namespace
TMPDIR=$(mktemp -d)
curl -sL https://raw.githubusercontent.com/thanos-community/thanos-operator/refs/heads/main/kustomization.yaml > "$TMPDIR/kustomization.yaml"
curl -sL https://raw.githubusercontent.com/thanos-community/thanos-operator/refs/heads/main/bundle.yaml > "$TMPDIR/bundle.yaml"
(cd $TMPDIR && kustomize edit set namespace $NAMESPACE) && kubectl create -k "$TMPDIR"
```

It can take some time for the operator to be up and running. You can wait for the operator's deployment to become available via,

```bash
kubectl wait --for=condition=Available deployment \
  -l app.kubernetes.io/component=manager,app.kubernetes.io/part-of=thanos-operator,control-plane=controller-manager \
  -n thanos-operator-system \
  --timeout=2m
```

You should now be able to create CRs!

## Create Custom Resources

You can take a look at the sample CRs we have within this repo in `config/samples`

To use those samples, you can run the following. This will create a [MinIO](https://min.io/) object storage and deploy the relevant Thanos component CRs in a [Receive-based](https://thanos.io/tip/components/receive.md/) setup.

```bash
git clone https://github.com/thanos-community/thanos-operator.git
kubectl config use-context <YOUR-CLUSTER>
make install-example
```

## Next Steps

After installation, proceed to the [Get Started guide](get-started.md) for a quick walkthrough of deploying Thanos components.
