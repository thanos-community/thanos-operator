# Introduction

Thanos Operator is a Kubernetes Operator that provides Kubernetes native deployment and management of Thanos and related monitoring components.

The Thanos operator includes, but is not limited to, the following features:

* **Kubernetes Custom Resources**: Allows using Kubernetes Custom Resources to deploy and manage Thanos Receive, Query, Query Frontend, Store Gateway, Compactor and Ruler. Provides simple and easy to understand configuration.
* **Gradually Adoptable**: This operator is designed to follow the same pattern of adoption as Thanos. Choose what Thanos components you need and would like to manage, and seamlessly integrate them into your current setup. Go from bare-bones simple setups to global scale monitoring.
* **Upstream Compatible**: It is built to be compatible with the latest [Thanos](https://thanos.io) releases, and to enhance and complement the [Prometheus Operator]("https://prometheus-operator.dev/"). Thanos Operator tries to follow similar configuration styles and ensures both operators can be used in tandem.

Thanos Operator provides a set of Custom Resource Definitions(CRDs) that allows you to configure your Thanos instances. Currently, the CRDs provided by Thanos Operator are:

- ThanosCompact
- ThanosQuery
- ThanosReceive
- ThanosRuler
- ThanosStore

## Goals

To significantly reduce the effort required to configure, implement and manage all components of a Thanos based monitoring stack.

* Ensure the operator is designed with consistent upstream-friendly design decisions and patterns
* Ensure the operator builds on top of the existing upstream ecosystem instead of redefining
* Ensure the operator provides a fully functional yet opinionated Thanos installation

## Next Steps

By now, you have the basic idea about Thanos Operator!!

Take a look at these guides to get into action with Thanos Operator.
