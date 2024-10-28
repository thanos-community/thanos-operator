package manifests

import (
	"fmt"
	"reflect"
	"testing"

	"k8s.io/utils/ptr"
)

func TestOptions_GetContainerImage(t *testing.T) {
	tests := []struct {
		name string
		o    Options
		want string
	}{
		{
			name: "get default image",
			o:    Options{},
			want: DefaultThanosImage + ":" + DefaultThanosVersion,
		},
		{
			name: "get custom image from options",
			o: Options{
				Image:   ptr.To("quay.io/thanos/thanos"),
				Version: ptr.To("latest"),
			},
			want: "quay.io/thanos/thanos:latest",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.o.GetContainerImage(); got != tt.want {
				t.Errorf("Options.GetContainerImage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOptions_ToFlags(t *testing.T) {
	tests := []struct {
		name string
		o    Options
		want []string
	}{
		{
			name: "get default flags",
			o:    Options{},
			want: []string{
				fmt.Sprintf("--log.level=%s", defaultLogLevel),
				fmt.Sprintf("--log.format=%s", defaultLogFormat),
			},
		},
		{
			name: "get custom flags",
			o: Options{
				LogLevel:  ptr.To("debug"),
				LogFormat: ptr.To("json"),
			},
			want: []string{
				"--log.level=debug",
				"--log.format=json",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if !reflect.DeepEqual(tt.o.ToFlags(), tt.want) {
				t.Errorf("Options.ToFlags() = %v, want %v", tt.o.ToFlags(), tt.want)
			}
		})
	}
}

func TestRelabelConfig_String(t *testing.T) {
	tests := []struct {
		name string
		r    RelabelConfig
		want string
	}{
		{
			name: "get value for hashmod",
			r: RelabelConfig{
				SourceLabel: "any",
				TargetLabel: "some_target",
				Modulus:     1,
				Action:      "hashmod",
			},
			want: `
- action: hashmod
  source_labels: ["any"]
  target_label: some_target
  modulus: 1`,
		},
		{
			name: "get value for keep",
			r: RelabelConfig{
				SourceLabel: "any",
				TargetLabel: "some_target",
				Regex:       "^test",
				Action:      "keep",
			},
			want: `
- action: keep
  source_labels: ["any"]
  target_label: some_target
  regex: ^test`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.r.String(); got != tt.want {
				t.Errorf("RelabelConfig.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRelabelConfigs_ToFlags(t *testing.T) {
	tests := []struct {
		name string
		rc   RelabelConfigs
		want string
	}{
		{
			name: "get flags for relabel configs",
			rc: RelabelConfigs{
				{
					SourceLabel: "any",
					TargetLabel: "some_target",
					Modulus:     1,
					Action:      "hashmod",
				},
				{
					SourceLabel: "any",
					TargetLabel: "some_target",
					Regex:       "^test",
					Action:      "keep",
				},
			},
			want: "--selector.relabel-config=" + `
- action: hashmod
  source_labels: ["any"]
  target_label: some_target
  modulus: 1
- action: keep
  source_labels: ["any"]
  target_label: some_target
  regex: ^test`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.rc.ToFlags(); got != tt.want {
				t.Errorf("RelabelConfigs.ToFlags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInMemoryCacheConfig_String(t *testing.T) {
	tests := []struct {
		name     string
		config   InMemoryCacheConfig
		expected string
	}{
		{
			name:   "EmptyConfig",
			config: InMemoryCacheConfig{},
			expected: `type: IN-MEMORY
config:
`,
		},
		{
			name:   "WithMaxSize",
			config: InMemoryCacheConfig{MaxSize: "100MB"},
			expected: `type: IN-MEMORY
config:
  max_size: 100MB
`,
		},
		{
			name:   "WithMaxItemSize",
			config: InMemoryCacheConfig{MaxItemSize: "10MB"},
			expected: `type: IN-MEMORY
config:
  max_item_size: 10MB
`,
		},
		{
			name:   "WithMaxSizeAndMaxItemSize",
			config: InMemoryCacheConfig{MaxSize: "100MB", MaxItemSize: "10MB"},
			expected: `type: IN-MEMORY
config:
  max_size: 100MB
  max_item_size: 10MB
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.config.String(); got != tt.expected {
				t.Errorf("InMemoryCacheConfig.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}
