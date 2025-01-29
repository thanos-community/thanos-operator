# Thanos Operator

[![Go Report Card](https://goreportcard.com/badge/github.com/thanos-community/thanos-operator)](https://goreportcard.com/report/github.com/thanos-community/thanos-operator) [![Go Code reference](https://img.shields.io/badge/code%20reference-go.dev-darkblue.svg)](https://pkg.go.dev/github.com/thanos-community/thanos-operator?tab=subdirectories) [![Static Badge](https://img.shields.io/badge/join_slack-%23thanos_operator-green)](https://cloud-native.slack.com/archives/C080V0HNV8W)

[![Build and Push Multi-Platform Image to Quay](https://github.com/thanos-community/thanos-operator/actions/workflows/push.yaml/badge.svg)](https://github.com/thanos-community/thanos-operator/actions/workflows/push.yaml) [![go](https://github.com/thanos-community/thanos-operator/actions/workflows/go.yml/badge.svg)](https://github.com/thanos-community/thanos-operator/actions/workflows/go.yml) [![docs](https://github.com/thanos-community/thanos-operator/actions/workflows/docs.yaml/badge.svg)](https://github.com/thanos-community/thanos-operator/actions/workflows/docs.yaml)

> [!NOTE]
>
> This operator is still a work in progress and does not provide API guarantees yet.

## Overview

The Thanos Operator provides Kubernetes native deployment and management of Thanos components. The purpose of this project is to simplify and automate the configuration of a Thanos based monitoring stack that would work across different topologies and deployment modes.

The Thanos operator includes, but is not limited to, the following features:

* **Kubernetes Custom Resources**: For Thanos Receive, Thanos Ruler, Thanos Store Gateway, Thanos Compactor and Thanos Querier. For Thanos Sidecar, please use [Prometheus CRD](https://prometheus-operator.dev/docs/api-reference/api/#monitoring.coreos.com/v1.Prometheus) from Prometheus-Operator.

* **Simplified Deployment Configuration**: Configure fundamental and advanced topologies wth simplified CRDs that can be composed into architecture of your choice.

* **Extensability with multi-cluster technologies**: Shares Thanos' philosophy of operating in multi-cluster environments. Provides extensible configuration so that you can easily manage Thanos installation with other multi-cluster technologies/initiatives like custom operators or [Open Cluster Management](https://open-cluster-management.io/).

For more details, read how to [get started](#getting-started) and explore our [CRD docs](./docs/api.md)

## Usage

We publish images from each commit to main and per-release at https://quay.io/thanos/thanos-operator

Thanos Operator binary CLI Options include,

```bash mdox-exec="./bin/manager --help"
Usage of ./bin/manager:
  -enable-http2
    	If set, HTTP/2 will be enabled for the metrics and webhook servers
  -feature-gate.enable-prometheus-operator-crds
    	If set, the operator will manage ServiceMonitors for components it deploys, and discover PrometheusRule objects to set on Thanos Ruler, from Prometheus Operator. (default true)
  -health-probe-bind-address string
    	The address the probe endpoint binds to. (default ":8081")
  -kubeconfig string
    	Paths to a kubeconfig. Only required if out-of-cluster.
  -leader-elect
    	Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.
  -metrics-bind-address string
    	The address the metric endpoint binds to. (default ":8080")
  -metrics-secure
    	If set the metrics endpoint is served securely
  -zap-devel
    	Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn). Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error)
  -zap-encoder value
    	Zap log encoding (one of 'json' or 'console')
  -zap-log-level value
    	Zap Level to configure the verbosity of logging. Can be one of 'debug', 'info', 'error', or any integer value > 0 which corresponds to custom debug levels of increasing verbosity
  -zap-stacktrace-level value
    	Zap Level at and above which stacktraces are captured (one of 'info', 'error', 'panic').
  -zap-time-encoding value
    	Zap time encoding (one of 'epoch', 'millis', 'nano', 'iso8601', 'rfc3339' or 'rfc3339nano'). Defaults to 'epoch'.
```

CRDs supported by this operator are defined in [./config/crd/bases](./config/crd/bases/). Operator deployment manifests are defined in [./config/manager](./config/manager/). To edit and build configuration refer to [CRD docs](./docs/api.md).

## Getting Started

To install CRDs run,

```bash
make install
```

To deploy operator run,

```bash
make deploy IMG="<IMAGE_NAME>"
```

IMAGE_NAME here is usually `example.com/thanos-operator:v0.0.1` for testing/local and `quay.io/thanos/thanos-operator:main-YYYY-MM-DD-COMMIT` (can reference latest from [quay](https://quay.io/thanos/thanos-operator))

To deploy [example manifests](./config/samples/), which give you a local [MinIO](https://min.io/) object storage instance and Thanos installation in Receive architecture, run,

```bash
make install-example
```

### Interactive Demo

To test out the operator with a local Kubernetes cluster you can run an interactive demo in KinD. You will firstly need to install [mage](https://magefile.org/) and then run the following commands:

```bash
mage interactiveDemo
```

Once the workloads are ready, run `kubectl -n thanos-operator-system port-forward svc/thanos-query-frontend-example-query 9090` and visit `http://localhost:9090` to via the query UI.

This demo consists of a Prometheus, deployed via Prometheus Operator, that scrapes metrics from the operator and the Thanos components themselves. It remote writes to the Thanos Receive component.

### Feature Gates

The CRDs within Thanos Operator have the ability to create/read certain Prometheus Operator objects, such as ServiceMonitors and PrometheusRules. However not all environments may have Prometheus Operator installed.

So the Thanos Operator binary and CRDs have flags to disable these using feature gates (they are enabled by default). You can set them using:
* `-feature-gate.enable-prometheus-operator-crds` flag on the binary
* `featureGates` on the relevant CRDs

## Contributing and development

Requirements to build, and test the project,

```
Go 1.22+
Linux or macOS
KinD 
kubectl
```

You can read about our goals and design decisions [here](./docs/DESIGN.md)!

Any contributions are welcome! Just use GitHub Issues and Pull Requests as usual. We follow [Thanos Go coding style guide](https://thanos.io/tip/contributing/coding-style-guide.md/).

Have questions or feedback? Join our slack channel [#thanos-operator](https://cloud-native.slack.com/archives/C080V0HNV8W)!

## Testing

The following `make` targets are available for testing:

1. `make test` - Runs unit and integration tests.
2. `make test-e2e` - Runs e2e tests against a Kubernetes cluster.

When executing integration tests, the following environment variables can be used to skip specific, per-controller tests:
* EXCLUDE_COMPACT=true
* EXCLUDE_QUERY=true
* EXCLUDE_RULER=true
* EXCLUDE_RECEIVE=true
* EXCLUDE_STORE=true

As an example, to run only integration tests for ThanosStore, you can run the following command:

```bash
EXCLUDE_COMPACT=true EXCLUDE_QUERY=true EXCLUDE_RULER=true EXCLUDE_RECEIVE=true make test
```

## Initial Authors

[@philipgough](https://github.com/PhilipGough) [@saswatamcode](https://github.com/saswatamcode)
