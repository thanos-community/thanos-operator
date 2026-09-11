# Feature Gates

Feature gates enable experimental Thanos Operator functionality. All features are disabled by default.

## Enabling and Configuring Features

Use `--enable-feature` for each feature you want to enable:

```bash
./thanos-operator \
  --enable-feature service-monitor \
  --enable-feature kube-resource-sync \
  --feature-gate-config-file /etc/thanos-operator/feature-gates.yaml
```

`--feature-gate-config-file` sets the YAML configuration path, which defaults to `/etc/thanos-operator/feature-gates.yaml`. The file configures features; it does not enable them. Omitted files, sections, or settings use defaults. Restart the operator after changing flags or configuration.

## ServiceMonitor Feature

**Flag**: `service-monitor`

Creates ServiceMonitors for Prometheus to scrape Thanos components. Requires Prometheus Operator and a Prometheus instance configured to discover the monitors.

```yaml
service-monitor:
  additionalLabels:
    prometheus: platform
  interval: 30s
```

These settings apply to all generated ServiceMonitors:

| Setting            | Default | Description                                                                                                                           |
|--------------------|---------|---------------------------------------------------------------------------------------------------------------------------------------|
| `additionalLabels` | `{}`    | Extra metadata labels for matching Prometheus's `spec.serviceMonitorSelector`. Existing workload and operator labels take precedence. |
| `interval`         | `""`    | Scrape interval. Empty uses Prometheus's global interval; otherwise, use a positive duration such as `30s` or `1m`.                   |

## PrometheusRule Feature

**Flag**: `prometheus-rule`

Lets ThanosRuler discover and use PrometheusRule resources for recording and alerting rules. Requires Prometheus Operator.

Configure discovery with `ThanosRuler.spec.ruleConfigSelector`. The default selector matches `operator.thanos.io/prometheus-rule: "true"`.

## OpenTelemetry Sidecar Feature

**Flag**: `otel-sidecar`

Enables OpenTelemetry collector sidecars for distributed tracing, with Thanos sending OTLP traces over HTTP to `localhost:4318`. Requires OpenTelemetry Operator and an [OpenTelemetryCollector configured for sidecar injection](https://opentelemetry.io/docs/kubernetes/operator/automatic/).

## KubeResourceSync Feature

**Flag**: `kube-resource-sync`

Adds a sidecar to ThanosReceive routers to synchronize hashring configuration changes immediately, avoiding kubelet ConfigMap update delays.

The `image` setting selects the container image. Its default is shown below:

```yaml
kube-resource-sync:
  image: quay.io/philipgough/kube-resource-sync:0.1.0
```

## Volume Resize Feature

**Flag**: `volume-resize`

Automatically expands persistent volumes when you increase storage sizes in Thanos custom resources. Requires a StorageClass with `allowVolumeExpansion: true`. Volumes can only grow, not shrink.

## Server TLS Feature (detailed review draft)

**Flag**: `server-tls`

This section documents the implemented behavior in detail for review. It includes lifecycle decisions, validation coverage, and limitations that can be condensed after review. Development-specific progress is also recorded in the local `FEATURE_GATE_NOTES.md` scratch file.

The server-tls gate configures server authentication and encryption for the HTTP and gRPC connections between operator-managed Thanos components. It applies to all managed components in the single namespace watched by the operator. The gate is disabled by default, and configuring its YAML block does not enable it.

### Requirements and enabling the feature

Install cert-manager before starting an operator with TLS enabled. The initial provider is `cert-manager`. Use Thanos v0.42.0 or newer; Query needs the [per-endpoint TLS configuration](https://github.com/thanos-io/thanos/pull/8594) introduced in that version. Image compatibility is the user's responsibility. The operator does not inspect image tags or digests to enforce a version requirement. Receive must use gRPC replication; its optional Cap'n Proto transport does not provide the TLS guarantees of this feature.

Initial TLS support requires one explicit `--watch-namespace`. The operator rejects an empty namespace or a list of namespaces when `server-tls` is enabled. This applies to automatic CA management and existing Issuer/ClusterIssuer configurations. The selected namespace must already exist; the operator does not create it. See [namespace scoping](./namespace-scoping.md) for deployment options.

For automatic certificate management, no YAML settings are necessary:

```bash
./thanos-operator --watch-namespace=monitoring --enable-feature=server-tls --enable-feature=service-monitor
```

ServiceMonitor is a separate gate. Enabling TLS alone does not enable monitoring. KubeResourceSync's separate metrics listener and ServiceMonitor keep their existing HTTP configuration; only the Thanos metrics endpoint switches to HTTPS. All components that communicate within a deployment need a consistent TLS setting. Existing external clients, such as a Prometheus remote-writing to Receive, also need to switch to HTTPS and trust the configured CA.

### Certificate provider and existing issuers

```yaml
server-tls:
  provider: cert-manager
  certManager:
    issuerRef:
      name: platform-ca
      kind: ClusterIssuer
    caBundleConfigMap:
      name: platform-trust
      key: ca.crt
```

`provider` selects the certificate integration and defaults to `cert-manager`. `issuerRef.kind` selects a Kubernetes resource type, not the provider. It accepts `Issuer` (the default) or `ClusterIssuer`. Its API group defaults to `cert-manager.io`. Other providers and issuer API groups are not supported yet.

The issuer signs the server certificates. The ConfigMap supplies the public PEM CA certificates clients trust. They are separate inputs because an issuer does not necessarily expose its trust chain in a form clients can consume. Supplying an existing issuer requires both `issuerRef` and `caBundleConfigMap`. The ConfigMap must exist in the selected namespace, including when using a ClusterIssuer. Put the PEM bundle in `data[key]`; `key` defaults to `ca.crt`. Mount the feature settings YAML into the operator at the path selected by `--feature-gate-config-file` and restart the operator after changing it.

In this mode, the operator creates leaf Certificate resources and references the supplied trust bundle. It does not manage the external issuer or overwrite the user's ConfigMap. The administrator must keep that bundle valid and ensure it trusts the selected issuer's certificates.

| YAML setting                                    | Default                                                     | Validation and scope                                                                           |
|-------------------------------------------------|-------------------------------------------------------------|------------------------------------------------------------------------------------------------|
| `server-tls.provider`                           | `cert-manager`                                              | Only `cert-manager` is currently accepted.                                                     |
| `server-tls.certManager.issuerRef.name`         | Automatic namespace issuer when both references are omitted | Required when an existing issuer is configured. Must be a valid Kubernetes resource name.      |
| `server-tls.certManager.issuerRef.kind`         | `Issuer`                                                    | Accepts `Issuer` or `ClusterIssuer`. A namespaced Issuer must exist in the selected namespace. |
| `server-tls.certManager.issuerRef.group`        | `cert-manager.io`                                           | Other issuer API groups are rejected.                                                          |
| `server-tls.certManager.caBundleConfigMap.name` | `thanos-operator-ca` in automatic mode                      | Required with an existing issuer. Looked up in the workload namespace.                         |
| `server-tls.certManager.caBundleConfigMap.key`  | `ca.crt`                                                    | Must be a valid ConfigMap data key containing public PEM CA certificates.                      |

The operator checks that the trust bundle is nonempty and contains parsable CA certificates. This check does not prove that the issuer is ready, that the bundle trusts a subsequently issued leaf, or that certificates are within their validity period. cert-manager reports issuance status on Certificate resources, and TLS clients enforce the trust chain, identity, and validity period when connecting.

### Automatic namespace CA setup

Omitting `certManager` selects automatic mode. The resource layout is:

| Resource                                      | Purpose                                                         |
|-----------------------------------------------|-----------------------------------------------------------------|
| `thanos-operator-selfsigned` Issuer           | Bootstraps a namespace CA certificate.                          |
| `thanos-operator-ca` Certificate and Secret   | Hold the namespace CA certificate and private key.              |
| `thanos-operator-ca` Issuer                   | Signs the Thanos server certificates using that CA.             |
| `thanos-operator-ca` ConfigMap                | Publishes only public CA certificates to clients.               |
| One Certificate and TLS Secret per workload   | Supply that workload's HTTP and gRPC server identity.           |
| One HTTP configuration ConfigMap per workload | Points Thanos's HTTP server at the mounted certificate and key. |

The leaf Certificate, Secret, and HTTP ConfigMap share the sanitized name `<workload>-tls`. For example, Deployment `thanos-query-example-query` uses `thanos-query-example-query-tls`. Separate shards get separate leaf resources. Query also gets an endpoint-discovery ConfigMap with its workload name, containing `endpoints.yaml`; this is independent of its HTTP configuration.

The TLS controller is registered when `server-tls` is enabled and queues bootstrap of the selected namespace when it starts. No ThanosQuery, ThanosReceive, ThanosStore, ThanosRuler, or ThanosCompact resource needs to exist first. Issuance remains asynchronous: the controller applies the CA resources, then publishes trust once cert-manager issues the CA Secret. It watches the shared CA resources to publish renewed trust and repair deleted resources. It does not watch Namespace objects or discover namespaces through component resources.

The shared namespace CA is retained independently of individual Thanos custom resources. Deleting one component must not remove trust used by the remaining components. Namespace trust reconciliation continues after the last component resource is deleted, for as long as this operator runs with TLS enabled. Leaf Certificates and HTTP ConfigMaps are owned by the Thanos custom resource. Component controllers use the existing resource pruner to remove them when TLS is disabled or a shard is removed. The TLS controller makes each issued leaf Secret a child of its Certificate, so Kubernetes garbage collection removes the private key with the Certificate even when cert-manager's optional Secret owner references are disabled. Name collisions with resources outside this integration produce an error. The operator checks its TLS management label, owner references for leaf resources, and cert-manager's certificate-name annotation on existing Secrets.

The CA private key is used by cert-manager and is never mounted into a Thanos pod or exposed through a ServiceMonitor. Thanos pods mount their own leaf key pair, the public trust ConfigMap, and the HTTP server configuration. Mounts use whole volumes rather than `subPath` so Kubernetes can deliver updated files.

### Connections covered

| Connection                                        | TLS configuration                                                                                  |
|---------------------------------------------------|----------------------------------------------------------------------------------------------------|
| Query -> Store, Receive, Ruler, or another Query  | Per-endpoint gRPC TLS with the namespace CA and an explicit Service DNS server name.               |
| Query Frontend -> Query                           | HTTPS downstream URL and a CA-backed HTTP transport.                                               |
| Ruler -> Query                                    | HTTPS discovery configuration with a CA and explicit Service DNS server name.                      |
| Stateless Ruler -> Receive                        | HTTPS remote-write destinations with CA verification.                                              |
| Receive router -> ingesters, including forwarding | gRPC TLS using the namespace CA; pod DNS names must be covered by the ingester certificate.        |
| Clients -> Receive remote-write listener          | HTTPS using the workload certificate.                                                              |
| Prometheus -> Thanos metrics                      | HTTPS ServiceMonitor endpoints with ConfigMap CA references and explicit Service DNS server names. |
| Kubelet -> Thanos health endpoints                | HTTPS probes. Kubernetes HTTPS probes do not verify the server certificate.                        |

All Thanos components, including Compactor and Query Frontend, serve their HTTP endpoints over TLS. Components exposing a gRPC server also use TLS for that server. Receive's remote-write HTTP listener is separate from its metrics listener and must be configured separately.

Receive hashring entries remain `<pod>.<service>.<namespace>.svc:10901` when TLS is enabled. These are gRPC targets, so they do not take an `https://` prefix. The hashring selects the destination; `--remote-write.client-tls-secure` enables TLS for forwarding and replication, and `--remote-write.client-tls-ca` supplies the CA bundle used to verify the destination's certificate. Despite their `remote-write` prefix, these client flags configure Receive's gRPC connections to other receivers. Thanos retains this naming for historical reasons. The destination enables its gRPC TLS listener with `--grpc-server-tls-cert` and `--grpc-server-tls-key`.

Incoming Prometheus or stateless Ruler remote write uses a separate HTTP endpoint, for example `https://<receive-service>.<namespace>.svc:19291/api/v1/receive`. That URL does need `https://`, and Receive secures that listener with `--remote-write.server-tls-cert` and `--remote-write.server-tls-key`. The flow is therefore client -> Receive over HTTPS, followed by Receive -> ingester over gRPC with TLS. A hashring address without a URL scheme does not indicate plaintext; the client and server flags determine the transport security.

This is server TLS, not mutual TLS: clients verify servers, and servers do not require client certificates. It does not add authorization. Object storage, Alertmanager, external caches, and user-supplied sidecars retain their own connection settings; the gate configures communication between managed Thanos components and their generated monitoring resources.

### DNS identity and service discovery

The canonical server identity is `<service>.<namespace>.svc`. External clients using a different connection address must still configure this TLS server name. Generated certificates do not include ingress hostnames or names ending in a cluster-specific DNS suffix. Query uses that name for certificate verification even when DNS SRV discovery resolves a different hostname or IP. Thanos v0.42 applies per-endpoint client settings only to static and group targets. Its regular DNS fanout path uses the global TLS configuration and loses the Service-specific server name. The live test exposed this as an IP SAN verification failure.

For normal fanout endpoints, the operator therefore watches EndpointSlices and emits one static target per ready address, including the endpoint port. Each target retains the original Service DNS name for certificate verification. It ignores unready endpoints, deduplicates overlapping slices, supports IPv6, and sorts targets so slice ordering does not cause unnecessary updates. The operator writes this list to a mounted ConfigMap. Query reloads it every five seconds once the kubelet projects the update, preserving fanout without restarting Query when endpoint membership changes. The first projected update may take up to the kubelet ConfigMap refresh interval.

Group endpoints keep DNS SRV discovery and round-robin behavior. Strict and strict-group endpoints keep their static Service addresses. There is no `clusterDomain` option.

Receive forwarding uses StatefulSet pod addresses. Ingester certificates also need to cover the pod addresses below their Service DNS name. Certificates must not contain a namespace-wide wildcard that lets one component impersonate every other service in that namespace.

The generated Receive certificate includes `<service>.<namespace>.svc` and `*.<service>.<namespace>.svc`. Other component certificates contain their Service DNS identity. Each namespace's automatic CA is separate. A discovered third-party StoreAPI Service must present a certificate trusted by the configured namespace bundle with the expected Service DNS name; discovery alone does not configure that service's TLS.

### Renewal, reloads, and migration

cert-manager issues and renews certificates. Thanos reloads server leaf certificates and private keys from their mounted files during fresh TLS handshakes. The operator does not roll workloads when a leaf Secret is first issued, renewed, or recreated. Secret volumes use whole-directory mounts, without `subPath`, so kubelet can project replacement files into running pods. Allow for kubelet projection before expecting a new connection to present the replacement certificate.

The reload paths checked against Thanos v0.42.0 are:

| Listener                                           | Reload behavior                                                                                                                |
|----------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------|
| Main HTTPS listener on each component              | Thanos uses exporter-toolkit v0.15.0, which reloads web TLS configuration and certificate files for new connections.           |
| gRPC listeners on Query, Receive, Store, and Ruler | Thanos's `GetCertificate` callback checks certificate/key file modification times during handshakes and reloads changed files. |
| Receive remote-write HTTPS listener                | Uses the same Thanos server TLS configuration and certificate callback as gRPC.                                                |

Existing connections continue using their established TLS session. Thanos's `--grpc-server-max-connection-age` defaults to 60 minutes, encouraging periodic reconnection and fresh handshakes; the operator leaves that default in place. Certificate renewal does not require client certificates or mutual TLS. See [Thanos's TLS implementation](https://github.com/thanos-io/thanos/blob/v0.42.0/pkg/tls/options.go), [Receive listener setup](https://github.com/thanos-io/thanos/blob/v0.42.0/cmd/thanos/receive.go), and [exporter-toolkit's reload callbacks](https://github.com/prometheus/exporter-toolkit/blob/v0.15.0/web/tls_config.go).

CA trust changes still trigger workload rollouts. Receive's forwarding gRPC client and Frontend's downstream HTTP transport load their root pools when their TLS configuration is constructed. A fresh connection alone does not reload those pools. The operator therefore hashes only the configured public CA bundle into `operator.thanos.io/tls-trust-checksum` and rolls all managed TLS workloads in that namespace when those bytes change. It uses one shared policy across components, including components without an outgoing TLS client. The checksum exists from initial workload creation, before leaf issuance, and stays unchanged when only a leaf certificate or key changes. Even a formatting-only change to the selected bundle data changes the checksum; unrelated ConfigMap keys do not.

The automatic CA has a ten-year duration and starts renewal one year before expiry. Its key is retained during renewal. The public ConfigMap accumulates previous roots so certificates issued before renewal remain trusted. Automatic pruning of historical roots is not implemented. Leaf private keys rotate on renewal; leaf validity uses cert-manager's default duration.

The existing Ruler rule reload sidecar sends its reload request to `https://localhost:9090/-/reload`. Its built-in HTTP client does not verify the server certificate. This exception is confined to the loopback connection inside the same pod; all generated connections between Thanos pods verify their server certificates. The reloader does not receive a client certificate or CA private key. A reloader with configurable CA verification would remove this limitation. Signal mode cannot be used here because that sidecar also checks a Prometheus-specific runtime API that Thanos does not expose.

Turning TLS on or off changes listener protocols and client settings and rolls workloads. Planned interruption is acceptable for this experimental feature. During migration, plaintext and TLS peers cannot communicate. Switch external clients and scrape configuration as part of the same migration. After changing a workload back to plaintext, the operator deletes its managed leaf Certificate, TLS Secret, and HTTP ConfigMap. The generated Query endpoint ConfigMap is also removed on disable. Shared namespace CA resources remain available for other workloads and later re-enablement. External issuers and trust ConfigMaps are retained. The live lifecycle test verified plaintext service after disabling TLS, then restored encrypted data flow using the same CA Secret UID.

### Validation results and coverage

Unit tests cover feature configuration and defaults, invalid provider/issuer settings, server flags and volume mounts, HTTPS probes and ServiceMonitors, endpoint discovery semantics, and the plaintext behavior when the gate is off. Fake-client reconciliation tests cover automatic CA setup, supplied issuers, ownership, certificate renewal, and trust changes.

The controller integration suite at `test/integration/featuregates/tls` checks resources written to an envtest API server with `server-tls` and `service-monitor` enabled. Each case starts a manager scoped to its own namespace, sharing one envtest API server across the suite. It covers namespace Issuers, the CA Certificate and public trust ConfigMap, leaf Certificates and HTTP ConfigMaps owned by their custom resource, Secret and ConfigMap mounts, server/client TLS flags, HTTPS probes, and ServiceMonitor CA/server-name settings. Component cases include Query and Frontend, both Receive roles, both Ruler modes, and separate Store and Compactor shards. Query uses a fixture Service and EndpointSlice to check its generated endpoint configuration; Ruler uses fixture discovery Services and a rule ConfigMap to check its HTTPS query, reload, and stateless remote-write settings.

Envtest runs the operator controllers but does not run cert-manager, kubelets, or Thanos. The suite supplies a generated CA Secret as a fixture and installs minimal Certificate/Issuer schemas that preserve their fields. It checks the operator's resource output, without claiming cert-manager admission validation or leaf issuance. Leaf Secret references are asserted in Certificates and pod volumes; actual leaf creation, renewal, and traffic remain E2E concerns. The cert-manager test schemas are loaded only when the TLS gate is enabled. The TLS suite uses Kubernetes v1.34.1 envtest assets. A trust-update case replaces the fixture CA, checks that the public bundle retains both roots, and verifies changed checksums on a Deployment and StatefulSet. A TLS controller case checks startup bootstrap, asynchronous CA publication, and repair of deleted trust resources without any component resource existing. The suite is included automatically in `make test-integration`.

The isolated suite at `test/e2e/core-tls` runs operators with `server-tls` and `service-monitor` enabled. It contains two independent ordered groups, each with its own namespace and namespace-scoped operator:

| File              | Namespace                   | Coverage                                                                                                                                                                                                          |
|-------------------|-----------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `core_test.go`    | `e2e-core-tls`              | A TLS replica of all 11 scenarios in `test/e2e/core/core_test.go`, plus three additional checks. Keeps the two Receive hashrings, tenant configuration, Query discovery, stateful Ruler, and recording-rule flow. |
| `tls_test.go`     | `e2e-core-tls-lifecycle`    | The six TLS lifecycle scenarios, including Frontend, Store, Compactor, stateless Ruler, reissuance, and migration.                                                                                                |
| `suite_test.go`   | Both                        | Version gating and Kubernetes client setup. Each ordered group creates and cleans up its own operator and namespace.                                                                                              |
| `helpers_test.go` | Explicit namespace per call | Verified HTTPS requests, remote write, discovery assertions, certificate checks, and Prometheus target checks.                                                                                                    |

The core copy preserves the original scenario names and order so reviewers can compare the two files directly. HTTP requests use the namespace CA and Service DNS identity. Query discovery assertions inspect its endpoint ConfigMap, including both ingesters' pod addresses and TLS server names. Ruler discovery assertions inspect its HTTPS query configuration. Receive's external label uses the namespace to avoid sharing object-storage identity with the plaintext suite. Both ingesters must become ready.

The additional core checks update a stateful Ruler rule to evaluate `sum(test_metric)` through Query, verify ready leaf Certificates and HTTPS health endpoints for all five core workloads, inspect their ServiceMonitor CA/server-name settings, and require five successful Prometheus scrape targets. The stateful rule check exercises Ruler -> Query HTTPS and Query -> Ruler gRPC; the original `vector(1)` rule alone does not require an HTTP query from Ruler.

The lifecycle group owns a separate operator because it disables and re-enables TLS. It cannot change the core group's feature settings or interfere with its resource counts. The package can run alongside `test/e2e/core`; it does not enable TLS in the plaintext suite. Keep the replicated core scenarios aligned when core coverage changes.

Both core variants allow three minutes for the first successful remote write. A correct hashring ConfigMap in the API does not mean the router has received the projected file yet. During validation, the plaintext router still had its initial `[{}]` file while the API already contained both hashrings; the earlier two-minute wait could time out. This changes the retry budget, while keeping the successful-write assertion and KubeResourceSync-disabled configuration.

Before the native leaf-reload change, all 20 scenarios passed on September 11, 2026 against Thanos v0.42.0, cert-manager v1.21.0, and Kubernetes v1.34.0 in a disposable Kind cluster (583.856 seconds). The original plaintext core suite also passed all 11 scenarios in that cluster. Both packages passed lint, and `core-tls` skipped all 20 scenarios on v0.41.0. The revised lifecycle assertions are:

| Scenario              | Live assertion                                                                                                                                                                                      |
|-----------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Bootstrap             | Start Query, Frontend, Receive router and ingester, Ruler, Store, and Compactor; create one CA and seven leaf Certificates.                                                                         |
| Verified data flow    | Remote-write a sample over HTTPS, evaluate it through Ruler and Query, and read the result through Frontend. Reject an incorrect TLS server name.                                                   |
| Rule reload           | Update the rule ConfigMap and observe the new recording rule's result through Frontend while Ruler serves HTTPS.                                                                                    |
| Monitoring            | Prometheus reports all seven generated Thanos scrape targets as up using the public CA.                                                                                                             |
| Leaf reissuance       | Recreate all seven leaf Secrets, verify the new certificates on 14 HTTP/gRPC listeners, keep pod templates, UIDs, container IDs, and restart counts unchanged, and verify fresh writes and queries. |
| Disable and re-enable | Remove leaf Certificates, serve plaintext, then recover encrypted data flow with the original CA Secret UID.                                                                                        |

The plaintext Query E2E suite also passed on both v0.41.0 and v0.42.0. The existing CI matrix includes both versions and discovers the TLS suite through `./test/e2e/...`; TLS scenarios skip on v0.41.0. Broad unit tests, the existing envtest integration suite, `go vet ./...`, `go build ./...`, and configured lint checks for the changed Go packages passed. A separate manifest regression test verifies that enabling KubeResourceSync with TLS leaves that sidecar's metrics on HTTP.

Live coverage uses automatic CA mode and ordinary Query fanout. Existing Issuer/ClusterIssuer configuration, CA bundle rollover, strict/group discovery modes, and IPv6 discovery are covered by unit or fake-client tests, without a separate live cluster scenario. Missing-Secret reissuance exercises cert-manager replacement and Thanos leaf reloads. It does not wait for scheduled renewal, force expiration of established gRPC sessions, simulate years of CA aging, or prove uninterrupted CA rotation. The test uses fresh verified handshakes to check the actual certificate served by each listener. The KubeResourceSync combination has manifest coverage, without a combined live test. The local Ruler reloader certificate-verification exception remains as described above.

### Observing issuance and troubleshooting

Check Certificate readiness in the workload namespace. In automatic mode, `thanos-operator-ca` must be issued before the operator can publish trust and create the workload resources. Leaf issuance is asynchronous, so a newly created pod may initially wait for its Secret. A missing cert-manager installation prevents a TLS-enabled operator from setting up its certificate watches.

For an existing issuer, first check its readiness and the referenced ConfigMap's namespace and data key. An empty or invalid CA bundle produces a reconciliation error. A well-formed but unrelated CA bundle passes configuration checks and then causes client certificate verification failures. The operator does not silently switch generated clients to plaintext or disable verification on these failures.

For Query connection errors, compare the generated endpoint ConfigMap with the selected Services' ready EndpointSlice addresses, and check that each `client_config.server_name` matches its Service certificate. Allow time for kubelet projection followed by Query's five-second reload interval after endpoint changes. For leaf renewal, allow for Secret projection and inspect the certificate on a fresh verified connection. The `operator.thanos.io/tls-trust-checksum` annotation should stay unchanged for leaf updates and change when the configured CA bundle changes. Avoid deleting the shared CA to repair a leaf issuance problem: existing peers and external clients may still depend on that CA.

Local E2E commands must explicitly select a disposable kubeconfig. The suite changes its own operator's feature flags during lifecycle testing and creates and deletes its test namespace.

### Implementation details for review

The component builders configure TLS flags and mounts and return each workload's leaf Certificate and HTTP ConfigMap through `manifests.AppendTLSResources`. Cert-manager issues the leaf Secret referenced by the workload.

The TLS controller follows the other controllers' `syncResources` pattern. It applies the shared CA and issuers defined by `manifests.BuildNamespaceTLSResources`, reads the issued CA Secret, and publishes the public trust bundle while retaining previous roots. Startup and resource watches enqueue the same request for the configured namespace. With a supplied issuer, it validates the configured public trust ConfigMap without provisioning shared resources.

Component controllers check that Receive uses gRPC replication, read and validate the published trust bundle, and add its checksum to the desired workload before application. The generic resource handler applies all builder resources with the Thanos custom resource as owner. It has no TLS feature configuration or lifecycle logic. Trust reading and validation live in private controller helpers. A missing or invalid trust bundle causes workload reconciliation to retry. Initial issuance is asynchronous, so pods can wait for their leaf Secret after the namespace CA is ready.

TLS-enabled workload controllers watch leaf Certificates, TLS-labelled leaf Secrets and ConfigMaps, and the configured public trust ConfigMap. Shared CA Certificate, CA Secret, and Issuer events are handled by the TLS controller. Publishing a changed trust bundle requeues Thanos resources in the same namespace. Only the public trust bundle is hashed onto the pod template. The TLS controller also watches managed leaf Certificates and Secrets to maintain Certificate ownership of issued Secrets. Those events do not cause a rollout while trust is unchanged. These cert-manager watches are absent when the feature is disabled.

RBAC adds namespaced Certificate and Issuer management. Referencing a ClusterIssuer does not require the Thanos operator to read or modify ClusterIssuers; cert-manager resolves that reference. No Namespace watch or additional Namespace permissions are needed. No CA key is required by Thanos or Prometheus.

The Go dependency uses cert-manager v1.19.4 API types to limit unrelated dependency upgrades. This is independent of the cert-manager controller version installed in the cluster; the existing E2E setup installs cert-manager v1.21.0.
