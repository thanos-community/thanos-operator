package ruler

import (
	"reflect"
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	manifestsstore "github.com/thanos-community/thanos-operator/internal/pkg/manifests/store"
	"github.com/thanos-community/thanos-operator/test/utils"

	"gotest.tools/v3/golden"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	someCustomLabelValue string = "xyz"
	someOtherLabelValue  string = "abc"
)

func TestBuildRuler(t *testing.T) {
	opts := Options{
		Options: manifests.Options{
			Namespace: "ns",
			Owner:     "any",
			Image:     ptr.To("some-custom-image"),
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
			PodDisruptionConfig: &manifests.PodDisruptionBudgetOptions{},
		},
		Endpoints: []Endpoint{
			{
				ServiceName: "test-query",
				Namespace:   "ns",
				Port:        19101,
			},
		},
		RuleFiles: []corev1.ConfigMapKeySelector{
			{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "test-rules",
				},
				Key: "rules.yaml",
			},
		},
		ObjStoreSecret: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: "test-secret",
			},
			Key: "thanos.yaml",
		},
		Retention:       "15d",
		AlertmanagerURL: "http://test-alertmanager.com:9093",
		ExternalLabels: map[string]string{
			"rule_replica": "0",
		},
	}

	objs := opts.Build()
	if len(objs) != 4 {
		t.Fatalf("expected 3 objects, got %d", len(objs))
	}

	utils.ValidateIsNamedServiceAccount(t, objs[0], opts, opts.Namespace)
	expectStateful := NewRulerStatefulSet(opts)
	utils.ValidateObjectsEqual(t, objs[1], expectStateful)
	utils.ValidateObjectsEqual(t, objs[2], NewRulerService(opts))
	if objs[3].GetObjectKind().GroupVersionKind().Kind != "PodDisruptionBudget" {
		t.Errorf("expected object to be a PodDisruptionBudget, got %v", objs[3].GetObjectKind().GroupVersionKind().Kind)
	}
	utils.ValidateLabelsMatch(t, objs[3], objs[1])

	wantLabels := opts.GetSelectorLabels()
	wantLabels["some-custom-label"] = someCustomLabelValue
	wantLabels["some-other-label"] = someOtherLabelValue
	wantLabels = manifests.MergeLabels(wantLabels, manifestsstore.GetRequiredStoreServiceLabel())
	utils.ValidateObjectLabelsEqual(t, wantLabels, []client.Object{objs[1], objs[2]}...)
}

func TestNewRulerStatefulSet(t *testing.T) {

	for _, tc := range []struct {
		name   string
		golden string
		opts   Options
	}{
		{
			name:   "test ruler statefulset correctness",
			golden: "statefulset-basic.golden.yaml",
			opts: Options{
				Options: manifests.Options{
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Annotations: map[string]string{
						"test": "annotation",
					},
				},
				Endpoints: []Endpoint{
					{
						ServiceName: "test-query",
						Namespace:   "ns",
						Port:        19101,
					},
				},
				RuleFiles: []corev1.ConfigMapKeySelector{
					{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "test-rules",
						},
						Key: "rules.yaml",
					},
				},
				ObjStoreSecret: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "test-secret",
					},
					Key: "thanos.yaml",
				},
				Retention:       "15d",
				AlertmanagerURL: "http://test-alertmanager.com:9093",
				ExternalLabels: map[string]string{
					"rule_replica": "0",
				},
			},
		},
		{
			name:   "test additional volumemount",
			golden: "statefulset-with-volumemount.golden.yaml",
			opts: Options{
				Options: manifests.Options{
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Annotations: map[string]string{
						"test": "annotation",
					},
					Additional: manifests.Additional{
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "some-rule",
								MountPath: "/some-rule",
							},
						},
					},
				},
				Endpoints: []Endpoint{
					{
						ServiceName: "test-query",
						Namespace:   "ns",
						Port:        19101,
					},
				},
				RuleFiles: []corev1.ConfigMapKeySelector{
					{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "test-rules",
						},
						Key: "rules.yaml",
					},
				},
				ObjStoreSecret: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "test-secret",
					},
					Key: "thanos.yaml",
				},
				Retention:       "15d",
				AlertmanagerURL: "http://test-alertmanager.com:9093",
				ExternalLabels: map[string]string{
					"rule_replica": "0",
				},
			},
		},
		{
			name:   "test additional container",
			golden: "statefulset-with-container.golden.yaml",
			opts: Options{
				Options: manifests.Options{
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Annotations: map[string]string{
						"test": "annotation",
					},
					Additional: manifests.Additional{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test-image:latest",
								Args:  []string{"--test-arg"},
								Env: []corev1.EnvVar{{
									Name:  "TEST_ENV",
									Value: "test",
								}},
							},
						},
					},
				},
				Endpoints: []Endpoint{
					{
						ServiceName: "test-query",
						Namespace:   "ns",
						Port:        19101,
					},
				},
				RuleFiles: []corev1.ConfigMapKeySelector{
					{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "test-rules",
						},
						Key: "rules.yaml",
					},
				},
				ObjStoreSecret: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "test-secret",
					},
					Key: "thanos.yaml",
				},
				Retention:       "15d",
				AlertmanagerURL: "http://test-alertmanager.com:9093",
				ExternalLabels: map[string]string{
					"rule_replica": "0",
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ruler := NewRulerStatefulSet(tc.opts)

			// Test against golden file
			yamlBytes, err := yaml.Marshal(ruler)
			if err != nil {
				t.Fatalf("failed to marshal statefulset to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tc.golden)
		})
	}
}

func TestNewRulerService(t *testing.T) {

	opts := Options{
		Options: manifests.Options{
			Namespace: "ns",
			Image:     ptr.To("some-custom-image"),
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
		},
		Endpoints: []Endpoint{
			{
				ServiceName: "test-query",
				Namespace:   "ns",
				Port:        19101,
			},
		},
		RuleFiles: []corev1.ConfigMapKeySelector{
			{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "test-rules",
				},
				Key: "rules.yaml",
			},
		},
		ObjStoreSecret: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: "test-secret",
			},
			Key: "thanos.yaml",
		},
		Retention:       "15d",
		AlertmanagerURL: "http://test-alertmanager.com:9093",
		ExternalLabels: map[string]string{
			"rule_replica": "0",
		},
	}

	ruler := NewRulerService(opts)

	// Test against golden file
	yamlBytes, err := yaml.Marshal(ruler)
	if err != nil {
		t.Fatalf("failed to marshal service to YAML: %v", err)
	}
	golden.Assert(t, string(yamlBytes), "service-basic.golden.yaml")
}

func TestGenerateRuleFileContent(t *testing.T) {
	duration10m := monitoringv1.Duration("10m")
	tests := []struct {
		name     string
		groups   []monitoringv1.RuleGroup
		expected string
	}{
		{
			name: "basic rule group",
			groups: []monitoringv1.RuleGroup{
				{
					Name: "example",
					Rules: []monitoringv1.Rule{
						{
							Alert: "HighRequestLatency",
							Expr:  intstr.IntOrString{Type: intstr.String, StrVal: "job:request_latency_seconds:mean5m{job=\"myjob\"} > 0.5"},
							For:   &duration10m,
							Labels: map[string]string{
								"severity": "page",
							},
						},
					},
				},
			},
			expected: `groups:
- name: example
  rules:
  - alert: HighRequestLatency
    expr: job:request_latency_seconds:mean5m{job="myjob"} > 0.5
    for: 10m
    labels:
      severity: page
`,
		},
		{
			name:     "empty groups",
			groups:   []monitoringv1.RuleGroup{},
			expected: "groups: []\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateRuleFileContent(tt.groups)
			if got != tt.expected {
				t.Errorf("GenerateRuleFileContent() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMakeRulesConfigMaps(t *testing.T) {
	tests := []struct {
		name      string
		ruleFiles map[string]string
		want      []corev1.ConfigMap
		wantErr   bool
	}{
		{
			name: "single rule file within size limit",
			ruleFiles: map[string]string{
				"test.yaml": `groups:
- name: example
  rules:
  - alert: HighRequestLatency
    expr: job:request_latency_seconds:mean5m{job="myjob"} > 0.5
    for: 10m
    labels:
      severity: page`,
			},
			want: []corev1.ConfigMap{
				{
					Data: map[string]string{
						"test.yaml": `groups:
- name: example
  rules:
  - alert: HighRequestLatency
    expr: job:request_latency_seconds:mean5m{job="myjob"} > 0.5
    for: 10m
    labels:
      severity: page`,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple rule files within size limit",
			ruleFiles: map[string]string{
				"test1.yaml": `groups:
- name: example1
  rules:
  - alert: Alert1
    expr: metric1 > 0.5`,
				"test2.yaml": `groups:
- name: example2
  rules:
  - alert: Alert2
    expr: metric2 > 0.5`,
			},
			want: []corev1.ConfigMap{
				{
					Data: map[string]string{
						"test1.yaml": `groups:
- name: example1
  rules:
  - alert: Alert1
    expr: metric1 > 0.5`,
						"test2.yaml": `groups:
- name: example2
  rules:
  - alert: Alert2
    expr: metric2 > 0.5`,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "rule file exceeding size limit",
			ruleFiles: map[string]string{
				"large.yaml": string(make([]byte, maxConfigMapDataSize+1)),
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MakeRulesConfigMaps(tt.ruleFiles)
			if (err != nil) != tt.wantErr {
				t.Errorf("MakeRulesConfigMaps() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MakeRulesConfigMaps() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBucketSize(t *testing.T) {
	tests := []struct {
		name   string
		bucket map[string]string
		want   int
	}{
		{
			name:   "empty bucket",
			bucket: map[string]string{},
			want:   0,
		},
		{
			name: "single file bucket",
			bucket: map[string]string{
				"test.yaml": "content",
			},
			want: 7,
		},
		{
			name: "multiple files bucket",
			bucket: map[string]string{
				"test1.yaml": "content1",
				"test2.yaml": "content2",
			},
			want: 16,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bucketSize(tt.bucket); got != tt.want {
				t.Errorf("bucketSize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildConfigReloaderContainer(t *testing.T) {
	tests := []struct {
		name              string
		opts              Options
		wantContainerName string
		wantImage         string
		wantArgsCount     int
		wantVolumeMounts  int
		wantPorts         int
	}{
		{
			name: "single rule file",
			opts: Options{
				RuleFiles: []corev1.ConfigMapKeySelector{
					{
						LocalObjectReference: corev1.LocalObjectReference{Name: "rules-1"},
						Key:                  "rule1.yaml",
					},
				},
				ConfigReloaderImage: "test-image:v1.0.0",
			},
			wantContainerName: "config-reloader",
			wantImage:         "test-image:v1.0.0",
			wantArgsCount:     3, // --listen-address, --reload-url, --watched-dir
			wantVolumeMounts:  1,
			wantPorts:         1,
		},
		{
			name: "multiple rule files from same ConfigMap",
			opts: Options{
				RuleFiles: []corev1.ConfigMapKeySelector{
					{
						LocalObjectReference: corev1.LocalObjectReference{Name: "rules-1"},
						Key:                  "rule1.yaml",
					},
					{
						LocalObjectReference: corev1.LocalObjectReference{Name: "rules-1"},
						Key:                  "rule2.yaml",
					},
				},
				ConfigReloaderImage: "test-image:v1.0.0",
			},
			wantContainerName: "config-reloader",
			wantImage:         "test-image:v1.0.0",
			wantArgsCount:     3, // Should deduplicate ConfigMap
			wantVolumeMounts:  1, // Only one mount for the single ConfigMap
			wantPorts:         1,
		},
		{
			name: "multiple rule files from different ConfigMaps",
			opts: Options{
				RuleFiles: []corev1.ConfigMapKeySelector{
					{
						LocalObjectReference: corev1.LocalObjectReference{Name: "rules-1"},
						Key:                  "rule1.yaml",
					},
					{
						LocalObjectReference: corev1.LocalObjectReference{Name: "rules-2"},
						Key:                  "rule2.yaml",
					},
					{
						LocalObjectReference: corev1.LocalObjectReference{Name: "rules-3"},
						Key:                  "rule3.yaml",
					},
				},
				ConfigReloaderImage: "test-image:v1.0.0",
			},
			wantContainerName: "config-reloader",
			wantImage:         "test-image:v1.0.0",
			wantArgsCount:     5, // --listen-address, --reload-url, --watched-dir (x3)
			wantVolumeMounts:  3,
			wantPorts:         1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := buildConfigReloaderContainer(tt.opts)

			if container.Name != tt.wantContainerName {
				t.Errorf("container.Name = %v, want %v", container.Name, tt.wantContainerName)
			}

			if container.Image != tt.wantImage {
				t.Errorf("container.Image = %v, want %v", container.Image, tt.wantImage)
			}

			if len(container.Args) != tt.wantArgsCount {
				t.Errorf("len(container.Args) = %v, want %v, args: %v", len(container.Args), tt.wantArgsCount, container.Args)
			}

			if len(container.VolumeMounts) != tt.wantVolumeMounts {
				t.Errorf("len(container.VolumeMounts) = %v, want %v", len(container.VolumeMounts), tt.wantVolumeMounts)
			}

			if len(container.Ports) != tt.wantPorts {
				t.Errorf("len(container.Ports) = %v, want %v", len(container.Ports), tt.wantPorts)
			}

			// Verify reload URL is correctly set
			hasReloadURL := false
			for _, arg := range container.Args {
				if arg == "--reload-url=http://localhost:9090/-/reload" {
					hasReloadURL = true
					break
				}
			}
			if !hasReloadURL {
				t.Error("Missing --reload-url argument")
			}
		})
	}
}

func TestConfigReloaderDeduplication(t *testing.T) {
	opts := Options{
		RuleFiles: []corev1.ConfigMapKeySelector{
			{LocalObjectReference: corev1.LocalObjectReference{Name: "rules-1"}, Key: "rule1.yaml"},
			{LocalObjectReference: corev1.LocalObjectReference{Name: "rules-1"}, Key: "rule2.yaml"},
			{LocalObjectReference: corev1.LocalObjectReference{Name: "rules-2"}, Key: "rule3.yaml"},
			{LocalObjectReference: corev1.LocalObjectReference{Name: "rules-1"}, Key: "rule4.yaml"},
		},
		ConfigReloaderImage: "test-image:v1.0.0",
	}

	container := buildConfigReloaderContainer(opts)

	// Should only mount 2 unique ConfigMaps (rules-1 and rules-2)
	if len(container.VolumeMounts) != 2 {
		t.Errorf("Expected 2 volume mounts, got %d", len(container.VolumeMounts))
	}

	// Verify the correct ConfigMaps are mounted
	mountedConfigMaps := make(map[string]bool)
	for _, vm := range container.VolumeMounts {
		mountedConfigMaps[vm.Name] = true
	}

	if !mountedConfigMaps["rules-1"] || !mountedConfigMaps["rules-2"] {
		t.Errorf("Expected rules-1 and rules-2 to be mounted, got: %v", mountedConfigMaps)
	}

	// Verify watched directories
	watchedDirCount := 0
	for _, arg := range container.Args {
		if len(arg) > len("--watched-dir=") && arg[:14] == "--watched-dir=" {
			watchedDirCount++
		}
	}
	if watchedDirCount != 2 {
		t.Errorf("Expected 2 --watched-dir arguments, got %d", watchedDirCount)
	}
}

func TestStatefulSetWithConfigReloader(t *testing.T) {
	for _, tc := range []struct {
		name           string
		golden         string
		opts           Options
		expectReloader bool
		containerCount int
	}{
		{
			name:   "config-reloader enabled with rule files",
			golden: "statefulset-with-config-reloader.golden.yaml",
			opts: Options{
				Options: manifests.Options{
					Namespace: "ns",
					Image:     ptr.To("thanos:v0.30.0"),
				},
				Endpoints: []Endpoint{
					{ServiceName: "test-query", Namespace: "ns", Port: 19101},
				},
				RuleFiles: []corev1.ConfigMapKeySelector{
					{LocalObjectReference: corev1.LocalObjectReference{Name: "test-rules"}, Key: "rules.yaml"},
				},
				ObjStoreSecret:      corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "test-secret"}, Key: "thanos.yaml"},
				Retention:           "15d",
				AlertmanagerURL:     "http://test-alertmanager.com:9093",
				ConfigReloaderImage: "quay.io/prometheus-operator/prometheus-config-reloader:v0.89.0",
			},
			expectReloader: true,
			containerCount: 2,
		},
		{
			name:   "no config-reloader when no rule files",
			golden: "statefulset-without-rules.golden.yaml",
			opts: Options{
				Options: manifests.Options{
					Namespace: "ns",
					Image:     ptr.To("thanos:v0.30.0"),
				},
				Endpoints:           []Endpoint{{ServiceName: "test-query", Namespace: "ns", Port: 19101}},
				RuleFiles:           []corev1.ConfigMapKeySelector{}, // Empty rule files
				ObjStoreSecret:      corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "test-secret"}, Key: "thanos.yaml"},
				Retention:           "15d",
				AlertmanagerURL:     "http://test-alertmanager.com:9093",
				ConfigReloaderImage: "quay.io/prometheus-operator/prometheus-config-reloader:v0.89.0",
			},
			expectReloader: false, // Should not add config-reloader when there are no rule files
			containerCount: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sts := NewRulerStatefulSet(tc.opts)

			containers := sts.Spec.Template.Spec.Containers
			if len(containers) != tc.containerCount {
				t.Fatalf("Expected %d containers, got %d", tc.containerCount, len(containers))
			}

			hasReloader := false
			for _, container := range containers {
				if container.Name == configReloaderContainerName {
					hasReloader = true
					break
				}
			}

			if hasReloader != tc.expectReloader {
				t.Errorf("Expected config-reloader presence = %v, got %v", tc.expectReloader, hasReloader)
			}

			// Test against golden file
			yamlBytes, err := yaml.Marshal(sts)
			if err != nil {
				t.Fatalf("failed to marshal statefulset to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tc.golden)
		})
	}
}
