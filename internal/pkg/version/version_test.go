package version

import (
	"testing"

	"github.com/blang/semver/v4"
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    *semver.Version
	}{
		{
			name:    "standard version with v prefix",
			version: "v0.30.0",
			want:    ptr(semver.MustParse("0.30.0")),
		},
		{
			name:    "standard version without v prefix",
			version: "0.30.0",
			want:    ptr(semver.MustParse("0.30.0")),
		},
		{
			name:    "release candidate version",
			version: "v0.30.0-rc.1",
			want:    ptr(semver.MustParse("0.30.0-rc.1")),
		},
		{
			name:    "empty string",
			version: "",
			want:    nil,
		},
		{
			name:    "invalid version",
			version: "latest",
			want:    nil,
		},
		{
			name:    "version with build metadata",
			version: "v0.35.1+build.123",
			want:    ptr(semver.MustParse("0.35.1+build.123")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.version)
			if tt.want == nil {
				if got != nil {
					t.Errorf("Parse(%q) = %v, want nil", tt.version, got)
				}
				return
			}
			if got == nil {
				t.Errorf("Parse(%q) = nil, want %v", tt.version, tt.want)
				return
			}
			if !got.EQ(*tt.want) {
				t.Errorf("Parse(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestParseFromImage(t *testing.T) {
	tests := []struct {
		name  string
		image string
		want  *semver.Version
	}{
		{
			name:  "full image with version",
			image: "quay.io/thanos/thanos:v0.30.0",
			want:  ptr(semver.MustParse("0.30.0")),
		},
		{
			name:  "simple image with version",
			image: "thanos:v0.30.0",
			want:  ptr(semver.MustParse("0.30.0")),
		},
		{
			name:  "image with latest tag",
			image: "quay.io/thanos/thanos:latest",
			want:  nil,
		},
		{
			name:  "image without tag",
			image: "quay.io/thanos/thanos",
			want:  nil,
		},
		{
			name:  "empty image",
			image: "",
			want:  nil,
		},
		{
			name:  "image with port and version",
			image: "registry.example.com:5000/thanos:v0.35.0",
			want:  ptr(semver.MustParse("0.35.0")),
		},
		{
			name:  "image with custom tag",
			image: "custom/thanos:custom-build",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseFromImage(tt.image)
			if tt.want == nil {
				if got != nil {
					t.Errorf("ParseFromImage(%q) = %v, want nil", tt.image, got)
				}
				return
			}
			if got == nil {
				t.Errorf("ParseFromImage(%q) = nil, want %v", tt.image, tt.want)
				return
			}
			if !got.EQ(*tt.want) {
				t.Errorf("ParseFromImage(%q) = %v, want %v", tt.image, got, tt.want)
			}
		})
	}
}

func TestChecker_CheckFeature(t *testing.T) {
	logger := testr.New(t)

	tests := []struct {
		name          string
		imageVersion  string
		feature       Feature
		flagName      string
		wantSupported bool
	}{
		{
			name:          "supported feature with valid version at minimum",
			imageVersion:  "quay.io/thanos/thanos:v0.39.0",
			feature:       FeatureQueryPromQLEngine,
			flagName:      "--query.promql-engine",
			wantSupported: true,
		},
		{
			name:          "supported feature with version above minimum",
			imageVersion:  "quay.io/thanos/thanos:v0.40.0",
			feature:       FeatureQueryPromQLEngine,
			flagName:      "--query.promql-engine",
			wantSupported: true,
		},
		{
			name:          "baseline feature always enabled even with old version",
			imageVersion:  "quay.io/thanos/thanos:v0.35.0",
			feature:       FeatureQueryPromQLEngine, // baseline feature, no entry in MinVersionRequirements
			flagName:      "--query.promql-engine",
			wantSupported: true, // baseline features are always enabled
		},
		{
			name:          "unknown version allows all features",
			imageVersion:  "custom/thanos:latest",
			feature:       FeatureQueryPromQLEngine,
			flagName:      "--query.promql-engine",
			wantSupported: true,
		},
		{
			name:          "unknown feature",
			imageVersion:  "quay.io/thanos/thanos:v0.39.0",
			feature:       Feature("unknown-feature"),
			flagName:      "--unknown-flag",
			wantSupported: true, // unknown features are allowed (no entry in map)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker(logger, "test-component", "test-resource", tt.imageVersion)
			got := checker.CheckFeature(tt.feature, tt.flagName)
			if got != tt.wantSupported {
				t.Errorf("CheckFeature(%q, %q) = %v, want %v", tt.feature, tt.flagName, got, tt.wantSupported)
			}
		})
	}
}

func TestChecker_IsVersionKnown(t *testing.T) {
	logger := testr.New(t)

	tests := []struct {
		name      string
		image     string
		wantKnown bool
	}{
		{
			name:      "known version from image",
			image:     "quay.io/thanos/thanos:v0.35.0",
			wantKnown: true,
		},
		{
			name:      "unknown version - latest tag",
			image:     "quay.io/thanos/thanos:latest",
			wantKnown: false,
		},
		{
			name:      "unknown version - custom tag",
			image:     "custom/thanos:custom-build",
			wantKnown: false,
		},
		{
			name:      "version string directly",
			image:     "v0.35.0",
			wantKnown: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker(logger, "test-component", "test-resource", tt.image)
			got := checker.IsVersionKnown()
			if got != tt.wantKnown {
				t.Errorf("IsVersionKnown() = %v, want %v", got, tt.wantKnown)
			}
		})
	}
}

func TestNewChecker(t *testing.T) {
	logger := logr.Discard()

	tests := []struct {
		name           string
		imageOrVersion string
		wantVersionNil bool
	}{
		{
			name:           "full image with version",
			imageOrVersion: "quay.io/thanos/thanos:v0.39.0",
			wantVersionNil: false,
		},
		{
			name:           "version string",
			imageOrVersion: "v0.39.0",
			wantVersionNil: false,
		},
		{
			name:           "latest tag",
			imageOrVersion: "quay.io/thanos/thanos:latest",
			wantVersionNil: true,
		},
		{
			name:           "empty string",
			imageOrVersion: "",
			wantVersionNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker(logger, "test", "test", tt.imageOrVersion)
			if tt.wantVersionNil && checker.Version() != nil {
				t.Errorf("NewChecker(%q).Version() = %v, want nil", tt.imageOrVersion, checker.Version())
			}
			if !tt.wantVersionNil && checker.Version() == nil {
				t.Errorf("NewChecker(%q).Version() = nil, want non-nil", tt.imageOrVersion)
			}
		})
	}
}

func TestChecker_IsSupportedVersion(t *testing.T) {
	logger := logr.Discard()

	tests := []struct {
		name           string
		imageOrVersion string
		wantSupported  bool
	}{
		{
			name:           "version at minimum",
			imageOrVersion: "quay.io/thanos/thanos:v0.39.0",
			wantSupported:  true,
		},
		{
			name:           "version above minimum",
			imageOrVersion: "quay.io/thanos/thanos:v0.40.0",
			wantSupported:  true,
		},
		{
			name:           "version below minimum",
			imageOrVersion: "quay.io/thanos/thanos:v0.38.0",
			wantSupported:  false,
		},
		{
			name:           "unknown version (custom image)",
			imageOrVersion: "custom/thanos:latest",
			wantSupported:  true, // allow custom images
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker(logger, "test", "test", tt.imageOrVersion)
			got := checker.IsSupportedVersion()
			if got != tt.wantSupported {
				t.Errorf("IsSupportedVersion() = %v, want %v", got, tt.wantSupported)
			}
		})
	}
}

func TestMinSupportedVersion(t *testing.T) {
	expected := semver.MustParse("0.39.0")
	if !MinSupportedVersion.EQ(expected) {
		t.Errorf("MinSupportedVersion = %v, want %v", MinSupportedVersion, expected)
	}
}

// ptr is a helper function to get a pointer to a value
func ptr[T any](v T) *T {
	return &v
}
