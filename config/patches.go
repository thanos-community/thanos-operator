package config

// ManagerAuthProxyPatch creates the manager auth proxy patch
// DO NOT IMPORT. This is only used for maintaining https://github.com/thanos-community/thanos-operator/blob/main/config/default/manager_auth_proxy_patch.yaml.
func ManagerAuthProxyPatch() map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      ManagerName,
			"namespace": DefaultNamespace,
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []map[string]interface{}{
						{
							"name":  "kube-rbac-proxy",
							"image": DefaultAuthProxyImage,
							"securityContext": map[string]interface{}{
								"allowPrivilegeEscalation": false,
								"capabilities": map[string]interface{}{
									"drop": []string{"ALL"},
								},
							},
							"args": []string{
								"--secure-listen-address=0.0.0.0:8443",
								"--upstream=http://127.0.0.1:8080/",
								"--logtostderr=true",
								"--v=0",
							},
							"ports": []map[string]interface{}{
								{
									"containerPort": 8443,
									"protocol":      "TCP",
									"name":          "https",
								},
							},
							"resources": map[string]interface{}{
								"limits": map[string]interface{}{
									"cpu":    "500m",
									"memory": "128Mi",
								},
								"requests": map[string]interface{}{
									"cpu":    "5m",
									"memory": "64Mi",
								},
							},
						},
						{
							"name": "manager",
							"args": []string{
								"--health-probe-bind-address=:8081",
								"--metrics-bind-address=127.0.0.1:8080",
								"--leader-elect",
								"--log.format=logfmt",
								"--log.level=debug",
							},
						},
					},
				},
			},
		},
	}
}
