package config

// JSONPatchOperation represents a single JSON Patch operation following RFC 6902.
type JSONPatchOperation struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}

// MonitorTLSPatch creates a JSON Patch that configures the ServiceMonitor
// to use secure TLS configuration with certificates managed by cert-manager.
func MonitorTLSPatch() []JSONPatchOperation {
	return []JSONPatchOperation{
		{
			Op:   "replace",
			Path: "/spec/endpoints/0/tlsConfig",
			Value: map[string]any{
				"serverName":         "SERVICE_NAME.SERVICE_NAMESPACE.svc",
				"insecureSkipVerify": false,
				"ca": map[string]any{
					"secret": map[string]string{
						"name": "metrics-server-cert",
						"key":  "ca.crt",
					},
				},
				"cert": map[string]any{
					"secret": map[string]string{
						"name": "metrics-server-cert",
						"key":  "tls.crt",
					},
				},
				"keySecret": map[string]any{
					"name": "metrics-server-cert",
					"key":  "tls.key",
				},
			},
		},
	}
}

// CertMetricsManagerPatch creates a JSON Patch that adds the args, volumes,
// and volumeMounts to allow the manager to use the metrics-server certificates.
func CertMetricsManagerPatch() []JSONPatchOperation {
	return []JSONPatchOperation{
		{
			Op:   "add",
			Path: "/spec/template/spec/containers/0/volumeMounts/-",
			Value: map[string]any{
				"mountPath": "/tmp/k8s-metrics-server/metrics-certs",
				"name":      "metrics-certs",
				"readOnly":  true,
			},
		},
		{
			Op:    "add",
			Path:  "/spec/template/spec/containers/0/args/-",
			Value: "--metrics-cert-path=/tmp/k8s-metrics-server/metrics-certs",
		},
		{
			Op:   "add",
			Path: "/spec/template/spec/volumes/-",
			Value: map[string]any{
				"name": "metrics-certs",
				"secret": map[string]any{
					"secretName": "metrics-server-cert",
					"optional":   false,
					"items": []map[string]string{
						{
							"key":  "ca.crt",
							"path": "ca.crt",
						},
						{
							"key":  "tls.crt",
							"path": "tls.crt",
						},
						{
							"key":  "tls.key",
							"path": "tls.key",
						},
					},
				},
			},
		},
	}
}
