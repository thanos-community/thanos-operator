package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/sh"

	"github.com/thanos-community/thanos-operator/test/utils"
)

const (
	kindClusterName   = "thanos-operator-cluster"
	operatorNamespace = "thanos-operator-system"
	projectImage      = "quay.io/thanos/thanos-operator"
)

var Aliases = map[string]interface{}{
	"interactive-demo": InteractiveDemo,
}

// InteractiveDemo runs an interactive demo.
func InteractiveDemo() error {
	if err := kindCluster(); err != nil {
		return err
	}

	if err := os.Setenv("KIND_CLUSTER", kindClusterName); err != nil {
		return err
	}

	if err := prometheusOperator(); err != nil {
		return err
	}

	if err := configurePrometheusInteractiveDemo(); err != nil {
		return err
	}

	if err := createOperatorNamespace(); err != nil {
		return err
	}

	if err := minio(); err != nil {
		return err
	}

	if err := BuildAndLoadLocalImage(); err != nil {
		return err
	}

	if err := installCRDS(); err != nil {
		return err
	}

	if err := certManager(); err != nil {
		return err
	}

	if err := openTelemetryOperator(); err != nil {
		return fmt.Errorf("failed to install OpenTelemetry Operator: %w", err)
	}

	if err := configureSidecarCollector(); err != nil {
		return fmt.Errorf("failed to configure sidecar collector: %w", err)
	}

	if err := configureJaegerV2(); err != nil {
		return fmt.Errorf("failed to configure Jaeger v2: %w", err)
	}

	if err := installSamples(); err != nil {
		return err
	}

	if err := configurePrometheusRules(); err != nil {
		return err
	}

	log.Println("🎉 Interactive demo setup complete!")
	log.Println("")
	log.Println("📊 Access the UIs:")
	log.Println("  Thanos Query Frontend: kubectl -n thanos-operator-system port-forward svc/thanos-query-frontend-example-query 9090")
	log.Println("  Then visit: http://localhost:9090")
	log.Println("")
	log.Println("  Prometheus UI: kubectl -n default port-forward svc/prometheus-operated 9090")
	log.Println("  Then visit: http://localhost:9090")
	log.Println("")
	log.Println("🔍 Jaeger v2 Tracing UI: kubectl -n thanos-operator-system port-forward svc/jaeger-v2-ui 16686")
	log.Println("  Then visit: http://localhost:16686")
	log.Println("")
	log.Println("📈 Check metrics and tracing correlation:")
	log.Println("  - View traces in Jaeger to see request flow")
	log.Println("  - Query metrics in Thanos to see performance data")
	log.Println("  - OpenTelemetry collector handles both metrics and tracing")
	log.Println("")
	log.Println("🔧 Tracing Setup:")
	log.Println("  - OTLP endpoint: otel-collector-collector.thanos-operator-system.svc.cluster.local:4318")
	log.Println("  - Jaeger endpoint: otel-collector-collector.thanos-operator-system.svc.cluster.local:14250")
	log.Println("  - Send traces to collector using OTLP or Jaeger protocols")

	if err := KubeStateMetrics(); err != nil {
		return err
	}

	if err := Grafana(); err != nil {
		return err
	}
	return nil
}

// kindCluster creates a new KinD cluster named 'thanos-operator-cluster'
func kindCluster() error {
	o, err := sh.Output("kind", "get", "clusters")
	if err != nil {
		return fmt.Errorf("failed to get KinD clusters: %w", err)
	}

	if strings.Contains(o, kindClusterName) {
		log.Println("KinD cluster already exists, skipping creation...")
		return nil
	}

	return sh.Run("kind", "create", "cluster", "--name", kindClusterName)
}

// prometheusOperator installs the Prometheus Operator in the Kubernetes cluster
func prometheusOperator() error {
	return utils.InstallPrometheusOperator()
}

// minio installs MinIO in the Kubernetes cluster
func minio() error {
	err := utils.InstallMinIO()
	if err != nil {
		return err
	}
	return utils.CreateMinioObjectStorageSecret()
}

// BuildAndLoadLocalImage Builds the container image and loads it into the KinD cluster
func BuildAndLoadLocalImage() error {
	err := buildLocalImage()
	if err != nil {
		return err
	}
	return utils.LoadImageToKindClusterWithName(getImageName())
}

// buildLocalImage builds the container image
func buildLocalImage() error {
	return sh.Run("docker", "build", "--load", ".", "-t", getImageName())
}

// installCRDS applies the Thanos Operators CRDs into the Kubernetes cluster
func installCRDS() error {
	return sh.Run("kubectl", "apply", "--server-side", "-f", "config/crd/bases/")
}

// installSamples installs the Thanos Operator sample resources into the Kubernetes cluster
func installSamples() error {
	env := map[string]string{"IMG_MAIN": getImageName()}
	if err := sh.RunWith(env, "make", "install-sample"); err != nil {
		return err
	}
	return nil
}

func createOperatorNamespace() error {
	content := fmt.Sprintf(`
kind: Namespace
apiVersion: v1
metadata:
  name: %s
`, operatorNamespace)

	_, err := applyKubeResources(content)
	return err
}

func getImageName() string {
	x, err := sh.Output("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		panic(err)
	}
	y, err := sh.Output("date", "+%Y-%m-%d")
	if err != nil {
		panic(err)
	}
	z, err := sh.Output("git", "rev-parse", "--short", "HEAD")
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s:%s-%s-%s", projectImage, x, y, z)
}

func applyKubeResources(resources string, withArgs ...string) (string, error) {
	in := []byte(resources)
	dir, err := os.MkdirTemp("", "resources")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "tmpfile")
	if err := os.WriteFile(file, in, 0666); err != nil {
		return "", err
	}
	args := append([]string{"apply", "-f", file}, withArgs...)
	return sh.Output("kubectl", args...)
}

func applyNamespacedKubeResources(resources, namespace string) (string, error) {
	in := []byte(resources)
	dir, err := os.MkdirTemp("", "resources")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "tmpfile")
	if err := os.WriteFile(file, in, 0666); err != nil {
		return "", err
	}
	return sh.Output("kubectl", "-n", namespace, "apply", "-f", file)
}

func configurePrometheusInteractiveDemo() error {
	content := `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: prometheus
rules:
- apiGroups: [""]
  resources:
  - nodes
  - nodes/metrics
  - services
  - endpoints
  - pods
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources:
  - configmaps
  verbs: ["get"]
- apiGroups:
  - discovery.k8s.io
  resources:
  - endpointslices
  verbs: ["get", "list", "watch"]
- apiGroups:
  - networking.k8s.io
  resources:
  - ingresses
  verbs: ["get", "list", "watch"]
- nonResourceURLs: ["/metrics"]
  verbs: ["get"]
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: prometheus
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: prometheus
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: prometheus
subjects:
- kind: ServiceAccount
  name: prometheus
  namespace: default
---
apiVersion: monitoring.coreos.com/v1
kind: Prometheus
metadata:
  name: prometheus-dev
spec:
  serviceAccountName: prometheus
  scrapeInterval: 1s
  evaluationInterval: 1s
  serviceMonitorNamespaceSelector:
    matchExpressions:
    - {key: 'kubernetes.io/metadata.name', operator: In, values: [thanos-operator-system, kube-system]}
  serviceMonitorSelector:
    matchExpressions:
    - {key: 'app.kubernetes.io/part-of', operator: In, values: [thanos, thanos-operator]}
  remoteWrite:
    - url: http://thanos-receive-router-example-receive.thanos-operator-system.svc.cluster.local:19291/api/v1/receive
      name: thanos-receive-router-green
`
	_, err := applyNamespacedKubeResources(content, "default")
	return err
}

// certManager installs cert-manager which is required by both OpenTelemetry and Jaeger operators
func certManager() error {
	log.Println("Installing cert-manager...")

	// Check if cert-manager is already installed by looking for deployments
	if output, err := sh.Output("kubectl", "get", "deployment", "cert-manager", "-n", "cert-manager", "--ignore-not-found"); err == nil && strings.Contains(output, "cert-manager") {
		log.Println("cert-manager already installed, skipping...")
		return nil
	}

	log.Println("Installing cert-manager...")
	if err := sh.Run("kubectl", "apply", "-f", "https://github.com/cert-manager/cert-manager/releases/download/v1.14.1/cert-manager.yaml"); err != nil {
		return fmt.Errorf("failed to install cert-manager: %w", err)
	}

	// Wait for cert-manager to be ready
	log.Println("Waiting for cert-manager to be ready...")
	if err := sh.Run("kubectl", "wait", "--for=condition=available", "--timeout=300s", "deployment/cert-manager", "-n", "cert-manager"); err != nil {
		return fmt.Errorf("cert-manager not ready: %w", err)
	}
	if err := sh.Run("kubectl", "wait", "--for=condition=available", "--timeout=300s", "deployment/cert-manager-cainjector", "-n", "cert-manager"); err != nil {
		return fmt.Errorf("cert-manager-cainjector not ready: %w", err)
	}
	if err := sh.Run("kubectl", "wait", "--for=condition=available", "--timeout=300s", "deployment/cert-manager-webhook", "-n", "cert-manager"); err != nil {
		return fmt.Errorf("cert-manager-webhook not ready: %w", err)
	}

	return nil
}

// openTelemetryOperator installs the OpenTelemetry Operator in the Kubernetes cluster
func openTelemetryOperator() error {
	log.Println("Installing OpenTelemetry Operator...")

	// Check if OpenTelemetry operator is already installed by looking for deployments
	if output, err := sh.Output("kubectl", "get", "deployment", "opentelemetry-operator-controller-manager", "-n", "opentelemetry-operator-system", "--ignore-not-found"); err == nil && strings.Contains(output, "opentelemetry-operator-controller-manager") {
		log.Println("OpenTelemetry operator already installed, skipping...")
		return nil
	}

	log.Println("Installing OpenTelemetry operator...")
	if err := sh.Run("kubectl", "apply", "-f", "https://github.com/open-telemetry/opentelemetry-operator/releases/latest/download/opentelemetry-operator.yaml"); err != nil {
		return fmt.Errorf("failed to install OpenTelemetry operator: %w", err)
	}

	// Wait for OpenTelemetry operator to be ready
	log.Println("Waiting for OpenTelemetry operator to be ready...")
	if err := sh.Run("kubectl", "wait", "--for=condition=available", "--timeout=300s", "deployment/opentelemetry-operator-controller-manager", "-n", "opentelemetry-operator-system"); err != nil {
		return fmt.Errorf("OpenTelemetry operator not ready: %w", err)
	}

	return nil
}

// configureJaegerV2 creates a Jaeger v2 instance using OpenTelemetry collector
func configureJaegerV2() error {
	log.Println("Configuring Jaeger v2 with OpenTelemetry...")
	log.Println("Creating Jaeger v2 collector deployment...")
	content := `
apiVersion: opentelemetry.io/v1beta1
kind: OpenTelemetryCollector
metadata:
  name: jaeger-v2-instance
  namespace: thanos-operator-system
spec:
  image: jaegertracing/jaeger:latest
  ports:
  - name: jaeger-ui
    port: 16686
    targetPort: 16686
  - name: jaeger-grpc
    port: 14250
    targetPort: 14250
  config:
    service:
      extensions: [jaeger_storage, jaeger_query]
      pipelines:
        traces:
          receivers: [otlp]
          exporters: [jaeger_storage_exporter]
    extensions:
      jaeger_query:
        storage:
          traces: memstore
      jaeger_storage:
        backends:
          memstore:
            memory:
              max_traces: 100000
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
          http:
            endpoint: 0.0.0.0:4318
    exporters:
      jaeger_storage_exporter:
        trace_storage: memstore
---
apiVersion: v1
kind: Service
metadata:
  name: jaeger-v2-ui
  namespace: thanos-operator-system
  labels:
    app.kubernetes.io/name: jaeger-v2
    app.kubernetes.io/component: query
spec:
  type: NodePort
  ports:
  - port: 16686
    targetPort: 16686
    nodePort: 30686
    name: jaeger-ui
  selector:
    app.kubernetes.io/name: jaeger-v2-instance-collector
`
	_, err := applyNamespacedKubeResources(content, operatorNamespace)
	if err != nil {
		return fmt.Errorf("failed to create Jaeger v2 collector: %w", err)
	}

	log.Println("Successfully created Jaeger v2 collector")
	return nil
}

// configureSidecarCollector creates a default sidecar OpenTelemetryCollector for automatic injection
func configureSidecarCollector() error {
	log.Println("Configuring OpenTelemetry Sidecar Collector...")
	log.Println("Creating default sidecar collector for annotation sidecar.opentelemetry.io/inject=true...")
	content := `
apiVersion: opentelemetry.io/v1beta1
kind: OpenTelemetryCollector
metadata:
  name: default
  namespace: thanos-operator-system
spec:
  mode: sidecar
  config:
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
          http:
            endpoint: 0.0.0.0:4318

    processors:
      batch:
        timeout: 1s
        send_batch_size: 1024
      resource:
        attributes:
        - key: service.name
          value: "thanos"
          action: upsert

    exporters:
      otlp/jaeger:
        endpoint: jaeger-v2-instance-collector.thanos-operator-system.svc.cluster.local:4317
        tls:
          insecure: true

    service:
      pipelines:
        traces:
          receivers: [otlp]
          processors: [resource, batch]
          exporters: [otlp/jaeger]
`
	_, err := applyNamespacedKubeResources(content, operatorNamespace)
	if err != nil {
		return fmt.Errorf("failed to create sidecar collector: %w", err)
	}

	log.Println("Successfully created default sidecar collector")
	return nil
}

// configurePrometheusRules creates a PrometheusRule object with basic alerts
func configurePrometheusRules() error {
	content := `
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: thanos-operator-alerts
  namespace: thanos-operator-system
  labels:
    operator.thanos.io/prometheus-rule: "true"
    app.kubernetes.io/part-of: thanos-operator
spec:
  groups:
  - name: thanos-operator
    rules:
    - alert: ThanosOperatorPodDown
      expr: up{job="thanos-operator-metrics"} == 0
      for: 5m
      labels:
        severity: critical
      annotations:
        description: "Thanos Operator pod has been down for more than 5 minutes"
        summary: "Thanos Operator pod is down"
    - alert: ThanosQueryHighErrorRate
      expr: |
        sum(rate(grpc_server_handled_total{grpc_code=~"Unknown|Internal|Unavailable|DataLoss",job="thanos-query"}[5m]))
        /
        sum(rate(grpc_server_handled_total{job="thanos-query"}[5m])) > 0.05
      for: 5m
      labels:
        severity: warning
      annotations:
        description: "Thanos Query is experiencing high error rate"
        summary: "Thanos Query error rate > 5%"
`
	_, err := applyNamespacedKubeResources(content, operatorNamespace)
	return err
}

// KubeStateMetrics installs kube-state-metrics in the kube-system namespace
func KubeStateMetrics() error {
	content := `
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kube-state-metrics
  namespace: kube-system
  labels:
    app.kubernetes.io/name: kube-state-metrics
    app.kubernetes.io/version: 2.10.1
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kube-state-metrics
  labels:
    app.kubernetes.io/name: kube-state-metrics
    app.kubernetes.io/version: 2.10.1
rules:
- apiGroups: [""]
  resources:
  - configmaps
  - secrets
  - nodes
  - pods
  - services
  - serviceaccounts
  - persistentvolumeclaims
  - persistentvolumes
  - namespaces
  - endpoints
  verbs: ["list", "watch"]
- apiGroups: ["apps"]
  resources:
  - daemonsets
  - deployments
  - replicasets
  - statefulsets
  verbs: ["list", "watch"]
- apiGroups: ["batch"]
  resources:
  - cronjobs
  - jobs
  verbs: ["list", "watch"]
- apiGroups: ["autoscaling"]
  resources:
  - horizontalpodautoscalers
  verbs: ["list", "watch"]
- apiGroups: ["authentication.k8s.io"]
  resources:
  - tokenreviews
  verbs: ["create"]
- apiGroups: ["authorization.k8s.io"]
  resources:
  - subjectaccessreviews
  verbs: ["create"]
- apiGroups: ["policy"]
  resources:
  - poddisruptionbudgets
  verbs: ["list", "watch"]
- apiGroups: ["certificates.k8s.io"]
  resources:
  - certificatesigningrequests
  verbs: ["list", "watch"]
- apiGroups: ["discovery.k8s.io"]
  resources:
  - endpointslices
  verbs: ["list", "watch"]
- apiGroups: ["storage.k8s.io"]
  resources:
  - storageclasses
  - volumeattachments
  verbs: ["list", "watch"]
- apiGroups: ["admissionregistration.k8s.io"]
  resources:
  - mutatingwebhookconfigurations
  - validatingwebhookconfigurations
  verbs: ["list", "watch"]
- apiGroups: ["networking.k8s.io"]
  resources:
  - networkpolicies
  - ingressclasses
  - ingresses
  verbs: ["list", "watch"]
- apiGroups: ["coordination.k8s.io"]
  resources:
  - leases
  verbs: ["list", "watch"]
- apiGroups: ["rbac.authorization.k8s.io"]
  resources:
  - clusterrolebindings
  - clusterroles
  - rolebindings
  - roles
  verbs: ["list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kube-state-metrics
  labels:
    app.kubernetes.io/name: kube-state-metrics
    app.kubernetes.io/version: 2.10.1
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kube-state-metrics
subjects:
- kind: ServiceAccount
  name: kube-state-metrics
  namespace: kube-system
---
apiVersion: v1
kind: Service
metadata:
  name: kube-state-metrics
  namespace: kube-system
  labels:
    app.kubernetes.io/name: kube-state-metrics
    app.kubernetes.io/version: 2.10.1
spec:
  type: ClusterIP
  ports:
  - name: http-metrics
    port: 8080
    targetPort: http-metrics
    protocol: TCP
  - name: telemetry
    port: 8081
    targetPort: telemetry
    protocol: TCP
  selector:
    app.kubernetes.io/name: kube-state-metrics
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kube-state-metrics
  namespace: kube-system
  labels:
    app.kubernetes.io/name: kube-state-metrics
    app.kubernetes.io/version: 2.10.1
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: kube-state-metrics
  replicas: 1
  template:
    metadata:
      labels:
        app.kubernetes.io/name: kube-state-metrics
        app.kubernetes.io/version: 2.10.1
    spec:
      hostNetwork: false
      serviceAccountName: kube-state-metrics
      securityContext:
        fsGroup: 65534
        runAsGroup: 65534
        runAsNonRoot: true
        runAsUser: 65534
        seccompProfile:
          type: RuntimeDefault
      containers:
      - name: kube-state-metrics
        image: registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.10.1
        args:
        - --metric-annotations-allowlist=pods=[thanos.io/hashring-config-hash]
        ports:
        - name: http-metrics
          containerPort: 8080
          protocol: TCP
        - name: telemetry
          containerPort: 8081
          protocol: TCP
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 5
          timeoutSeconds: 5
        readinessProbe:
          httpGet:
            path: /
            port: 8081
          initialDelaySeconds: 5
          timeoutSeconds: 5
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          readOnlyRootFilesystem: true
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: kube-state-metrics
  namespace: kube-system
  labels:
    app.kubernetes.io/name: kube-state-metrics
    app.kubernetes.io/part-of: thanos
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: kube-state-metrics
  endpoints:
  - port: http-metrics
    interval: 30s
    path: /metrics
  - port: telemetry
    interval: 30s
    path: /metrics
`
	_, err := applyNamespacedKubeResources(content, "kube-system")
	return err
}

// Grafana installs Grafana with Thanos Query as a data source
func Grafana() error {
	content := `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: grafana-datasources
  namespace: default
  labels:
    app.kubernetes.io/name: grafana
data:
  datasources.yaml: |
    apiVersion: 1
    datasources:
    - name: Thanos Query
      type: prometheus
      access: proxy
      url: http://thanos-query-example-query.thanos-operator-system.svc.cluster.local:9090
      isDefault: true
      editable: true
    - name: Prometheus
      type: prometheus
      access: proxy
      url: http://prometheus-operated.default.svc.cluster.local:9090
      isDefault: false
      editable: true
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: grafana-config
  namespace: default
  labels:
    app.kubernetes.io/name: grafana
data:
  grafana.ini: |
    [analytics]
    check_for_updates = false
    [security]
    admin_user = admin
    admin_password = admin
    [server]
    root_url = http://localhost:3000/
    [users]
    allow_sign_up = false
    auto_assign_org = true
    auto_assign_org_role = Admin
    [auth.anonymous]
    enabled = false
---
apiVersion: v1
kind: Service
metadata:
  name: grafana
  namespace: default
  labels:
    app.kubernetes.io/name: grafana
spec:
  type: ClusterIP
  ports:
  - name: http
    port: 3000
    targetPort: 3000
    protocol: TCP
  selector:
    app.kubernetes.io/name: grafana
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: grafana
  namespace: default
  labels:
    app.kubernetes.io/name: grafana
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: grafana
  template:
    metadata:
      labels:
        app.kubernetes.io/name: grafana
    spec:
      securityContext:
        fsGroup: 472
        runAsGroup: 472
        runAsNonRoot: true
        runAsUser: 472
      containers:
      - name: grafana
        image: grafana/grafana:10.2.0
        ports:
        - name: http
          containerPort: 3000
          protocol: TCP
        env:
        - name: GF_SECURITY_ADMIN_USER
          value: admin
        - name: GF_SECURITY_ADMIN_PASSWORD
          value: admin
        - name: GF_INSTALL_PLUGINS
          value: ""
        volumeMounts:
        - name: config
          mountPath: /etc/grafana
          readOnly: true
        - name: datasources
          mountPath: /etc/grafana/provisioning/datasources
          readOnly: true
        - name: storage
          mountPath: /var/lib/grafana
        livenessProbe:
          httpGet:
            path: /api/health
            port: 3000
          initialDelaySeconds: 60
          timeoutSeconds: 30
        readinessProbe:
          httpGet:
            path: /api/health
            port: 3000
          initialDelaySeconds: 10
          timeoutSeconds: 30
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          readOnlyRootFilesystem: true
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 200m
            memory: 256Mi
      volumes:
      - name: config
        configMap:
          name: grafana-config
      - name: datasources
        configMap:
          name: grafana-datasources
      - name: storage
        emptyDir: {}
`
	_, err := applyNamespacedKubeResources(content, "default")
	return err
}
