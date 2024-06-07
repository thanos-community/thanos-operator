---
type: design
title: Thanos Operator Design Decision Record
owner: philipgough, saswatamcode
menu: proposals-accepted
---

# Thanos Operator Design Decision Record

## Summary

This document aims to specify the design for a metrics operator that enables multi-cluster observability use cases, backed by a [Thanos](https://thanos.io/) metrics backend.

## Why

There are a couple of main reasons for creating a new operator offering,

* Currently, no upstream option exists for managing Thanos installations on K8s.
* There is existing [Observatorium Operator offering](https://github.com/observatorium/operator), that makes use of a unique tool called [locutus](https://github.com/brancz/locutus). While this is functional and innovative, it relies on static configuration to render K8s objects, which leads to an un-customizable installer rather than an operator. Also, using a pattern that is not widely adopted, results in very low community adoption/contribution.

## Goals

* Ensure the operator is designed with consistent upstream-friendly design decisions and patterns
* Ensure the operator builds on top of the existing upstream ecosystem instead of redefining
* Ensure the operator provides a fully functional yet opinionated Thanos installation

## Non-Goals

* Creation and direct management of tenancy and authorization/authentication: Mechanisms for these need to be delegated to other technologies like Observatorium API, Keycloak, OpenPolicyAgent, and SpiceDB, and there are designs underway to facilitate this. Thanos Operator would only provide the tenancy primitives needed.
* Collection/scraping: This operator is designed to be a central metrics sink, collection would likely be delegated to things like Prometheus Operator, Otel Collector, etc.
* Multicluster operations: While this operator plays a crucial role in multicluster use cases, it will not be responsible for multicluster workload scheduling of any kind. Instead a separate design can be created using this operator as its base and leveraging open standard for multicluster management frameworks.
* Exhaustively supporting previous versions of Thanos where the latest v0.35.0 is the starting point.

## How

### Per Component Use Cases & Design

The following sections outline the identified use cases and stories per component and present one or more designs for discussion that cover those cases. Designs may change as we begin to develop the project if the proposed and accepted solutions become unviable for any reason.

### Metrics Ingest

### Metrics Query

#### Proposed Design: Query

We propose that there should be a single CRD for Thanos Query, as one end of the metrics query path.

_ThanosQuery_ will be responsible for managing the lifecycle of Querier deployments and handle automatically finding and connecting Thanos StoreAPI implementations to the Querier via labels/ownerreferences and restarting them if necessary. It will also allow for certain types of endpoint config such as HA grouping or strict depending on labels. 

**Option One - Endpoint flag-based Auto Service Discovery (always restarts)**

<img src="img/ThanosQueryOpt1.png" alt="Endpoint flag-based Auto Service Discovery" width="800"/>

**_“As a developer, I want to be able to spin up Thanos Query as a global view where I can see all my data, both in and out of cluster.”_**

This can be achieved by spinning up a ThanosQuery CR with minimal config, and deploying Services (that could be owned by other components in this Operator with required labels) or Endpoints objects that route to out-of-cluster addresses.

**_“As an operator of a multi-tenant metrics backend, I want to have queries that combine data from store, receive, and sidecar.”_**

This is doable as ThanosQuery will automatically discover Services with labels and use Thanos DNS notation to attach them as an endpoint using Query args, and restart Query for you.

**_“As an operator of a scalable metrics backend, I want to have queries that round robin series calls to HA groups of StoreAPI endpoints”_**

This is possible by adding a special label to services for your HA StoreAPI, which will cause the controller to configure them as endpoint-group flag. You can do the same for strict endpoints.

**_“As an operator of a multi-tenant metrics backend, I want to only attach certain StoreAPIs to particular Query deployments and provide separate QoS to different tenants.”_**

You can create individual ThanosQuery CRs with custom label selectors, so they only look for Services having those labels.

**_“As an operator of a multi-tenant metrics backend, I want to be able to run a distributed query engine for Thanos.”_**

While not planned for initial implementation, fields like _Distributed_ and _ShardingLabels_ can be added to the CR to allow it to spin up a query tree with distributed mode, which shards across labels you select.

**Option Two - File-based Auto Service Discovery (only restart for group/strict endpoint)**

<img src="img/ThanosQueryOpt2.png" alt="File-based Auto Service Discovery (only restart for group/strict endpoint)" width="800"/>

The only point of difference in this implementation is that we are using an SD file for endpoints. This ensures that Query pods don’t need to be restarted unless there is a change in endpoint-group or strict Services.

**_“As a developer, I want to be able to spin up Thanos Query as a global view where I can see all my data, both in and out of cluster.”_**

Same as prior implementation.

**_“As an operator of a multi-tenant metrics backend, I want to have queries that combine data from store, receive, and sidecar.”_**

Same as prior implementation.

**_“As an operator of a scalable metrics backend, I want to have queries that round robin series calls to HA groups of StoreAPI endpoints”_**

Same as prior implementation.

**_“As an operator of a multi-tenant metrics backend, I want to only attach certain StoreAPIs to particular Query deployments provide separate QoS to different tenants.”_**

Same as prior implementation.

**_“As an operator of a multi-tenant metrics backend, I want to be able to run a distributed query engine for Thanos.”_**

Same as prior implementation.

### Metrics Compaction

## Alternatives

### [Existing Thanos operator](https://github.com/banzaicloud/thanos-operator)

* There does not appear to be much in the way of active development 
* Thanos receive support exists but it is not well documented
* It appears to favor a sidecar-based approach
* No native support for multi-tenancy
* Unclear/poor documentation for how scaling is handled
* Doesn’t seem to support limits at the moment
* [No convenient way to setup rules](https://github.com/banzaicloud/thanos-operator/issues/173)
* A limited amount of tests seems to be in place
* We’d likely need to make major contributions to make this fit for purpose for us, and given the current level of activity in the project, it might be hard for these to land upstream. Further, the effort for these major contributions are likely to be in the same general size as creating an operator from scratch.

