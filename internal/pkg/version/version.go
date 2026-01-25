// Package version provides utilities for parsing and comparing Thanos versions,
// and checking feature compatibility based on version requirements.
package version

import (
	"strings"

	"github.com/blang/semver/v4"
	"github.com/go-logr/logr"
)

// MinSupportedVersion is the minimum Thanos version supported by this operator.
// All features available in this version or later are considered baseline and always enabled.
var MinSupportedVersion = semver.MustParse("0.39.0")

// Feature represents a Thanos feature that may have version requirements
type Feature string

// Define known features with version requirements.
// Only features introduced AFTER MinSupportedVersion (v0.39.0) need to be listed here.
// Features available in v0.39.0 or earlier are always enabled.
const (
	// Query features (all available in v0.39.0)
	FeatureQueryPromQLEngine         Feature = "query.promql-engine"
	FeatureQueryAutoDownsampling     Feature = "query.auto-downsampling"
	FeatureGRPCProxyStrategy         Feature = "grpc.proxy-strategy"
	FeatureQueryTelemetryQuantiles   Feature = "query.telemetry.quantiles"
	FeatureEndpointGroup             Feature = "endpoint-group"
	FeatureEndpointGroupStrict       Feature = "endpoint-group-strict"
	FeatureWebDisableCORS            Feature = "web.disable-cors"

	// Store features (all available in v0.39.0)
	FeatureStoreEnableIndexHeaderLazyReader Feature = "store.enable-index-header-lazy-reader"
	FeatureStoreIndexHeaderLazyDownload     Feature = "store.index-header-lazy-download-strategy"
	FeatureBlockDiscoveryStrategy           Feature = "block-discovery-strategy"
	FeatureStoreLimitsRequestSamples        Feature = "store.limits.request-samples"
	FeatureStoreLimitsRequestSeries         Feature = "store.limits.request-series"

	// Compact features (all available in v0.39.0)
	FeatureCompactBlockFetchConcurrency Feature = "compact.blocks-fetch-concurrency"
	FeatureCompactCleanupInterval       Feature = "compact.cleanup-interval"
	FeatureDownsampleConcurrency        Feature = "downsample.concurrency"
	FeatureBlockViewerGlobalSync        Feature = "block-viewer.global.sync"

	// Receive features (all available in v0.39.0)
	FeatureReceiveCapnProto              Feature = "receive.capnproto"
	FeatureReceiveReplicationProtocol    Feature = "receive.replication-protocol"
	FeatureReceiveAsyncForwardWorkers    Feature = "receive.forward.async-workers"
	FeatureReceiveTooFarInFuture         Feature = "tsdb.too-far-in-future.time-window"
	FeatureReceiveTenantCertificateField Feature = "receive.tenant-certificate-field"
	FeatureReceiveSplitTenantLabelName   Feature = "receive.split-tenant-label-name"
	FeatureReceiveGRPCCompression        Feature = "receive.grpc-compression"

	// Query Frontend features (all available in v0.39.0)
	FeatureQueryFrontendLabelsSplit     Feature = "labels.split-interval"
	FeatureQueryFrontendLabelsRetries   Feature = "labels.max-retries-per-request"
	FeatureQueryFrontendLabelsTimeRange Feature = "labels.default-time-range"
	FeatureCacheCompressionType         Feature = "cache-compression-type"

	// Ruler features (all available in v0.39.0)
	FeatureRulerEvalInterval Feature = "eval-interval"
	FeatureAlertLabelDrop    Feature = "alert.label-drop"
)

// MinVersionRequirements maps features to their minimum required Thanos version.
// Only features introduced AFTER v0.39.0 need entries here.
// All features listed above were introduced before v0.39.0 and are always available.
var MinVersionRequirements = map[Feature]semver.Version{
	// Add future features here that require versions > 0.39.0
	// Example:
	// FeatureNewFeature: semver.MustParse("0.40.0"),
}

// Parse parses a version string into a semver.Version.
// It accepts formats like "v0.30.0", "0.30.0", "v0.30.0-rc.1", etc.
// Returns nil if the version string cannot be parsed.
func Parse(versionStr string) *semver.Version {
	if versionStr == "" {
		return nil
	}

	// Strip leading 'v' if present
	versionStr = strings.TrimPrefix(versionStr, "v")

	v, err := semver.Parse(versionStr)
	if err != nil {
		// Try parsing with FinalizeVersion for more lenient parsing
		v, err = semver.ParseTolerant(versionStr)
		if err != nil {
			return nil
		}
	}

	return &v
}

// ParseFromImage extracts and parses version from a container image string.
// The image can be in formats like:
// - "quay.io/thanos/thanos:v0.30.0"
// - "thanos:v0.30.0"
// - "quay.io/thanos/thanos:v0.30.0-rc.1"
// Returns nil if no version can be extracted.
func ParseFromImage(image string) *semver.Version {
	if image == "" {
		return nil
	}

	// Find the tag after the colon
	idx := strings.LastIndex(image, ":")
	if idx == -1 || idx == len(image)-1 {
		return nil
	}

	tag := image[idx+1:]
	return Parse(tag)
}

// Checker provides version checking functionality with logging
type Checker struct {
	version *semver.Version
	logger  logr.Logger
	// componentName is used for logging context
	componentName string
	// resourceName is the name of the resource being checked (for logging)
	resourceName string
	// versionWarningLogged tracks if we've already logged the version warning
	versionWarningLogged bool
}

// NewChecker creates a new version checker.
// If version is nil or cannot be parsed, the checker will log warnings but allow all features.
func NewChecker(logger logr.Logger, componentName, resourceName, imageOrVersion string) *Checker {
	var version *semver.Version

	// Try to parse as a version string first
	version = Parse(imageOrVersion)
	if version == nil {
		// Try to extract from image
		version = ParseFromImage(imageOrVersion)
	}

	return &Checker{
		version:       version,
		logger:        logger,
		componentName: componentName,
		resourceName:  resourceName,
	}
}

// Version returns the parsed version, or nil if version couldn't be determined
func (c *Checker) Version() *semver.Version {
	return c.version
}

// IsVersionKnown returns true if the version could be determined
func (c *Checker) IsVersionKnown() bool {
	return c.version != nil
}

// CheckFeature checks if a feature is supported by the current version.
// If the version is unknown (custom image), it returns true (allow the feature).
// If the version is known but doesn't support the feature, it logs a warning and returns false.
// If the feature is supported, it returns true.
// Note: All features available in v0.39.0 (MinSupportedVersion) are always enabled.
func (c *Checker) CheckFeature(feature Feature, flagName string) bool {
	minVersion, hasRequirement := MinVersionRequirements[feature]
	if !hasRequirement {
		// No version requirement in the map means the feature was available before v0.39.0
		// Since we only support v0.39.0+, the feature is always available
		return true
	}

	if c.version == nil {
		// Version couldn't be determined (likely custom image)
		// Allow the feature but log a warning
		c.logger.Info(
			"Unable to determine Thanos version, cannot validate feature compatibility. Feature will be enabled but may not work if your Thanos version doesn't support it.",
			"component", c.componentName,
			"resource", c.resourceName,
			"feature", string(feature),
			"flag", flagName,
			"minRequiredVersion", minVersion.String(),
		)
		return true
	}

	if c.version.LT(minVersion) {
		c.logger.Info(
			"Feature requires a newer Thanos version. Skipping flag - upgrade Thanos or remove the configuration.",
			"component", c.componentName,
			"resource", c.resourceName,
			"feature", string(feature),
			"flag", flagName,
			"currentVersion", c.version.String(),
			"minRequiredVersion", minVersion.String(),
		)
		return false
	}

	return true
}

// WarnVersionUnknown logs a warning if the version couldn't be determined or is unsupported.
// This should be called once at the start of building args.
func (c *Checker) WarnVersionUnknown() {
	if c.versionWarningLogged {
		return
	}
	c.versionWarningLogged = true

	if c.version == nil {
		c.logger.Info(
			"Unable to determine Thanos version from image. Using custom image or non-standard tag. Feature compatibility checks will be skipped.",
			"component", c.componentName,
			"resource", c.resourceName,
			"minSupportedVersion", MinSupportedVersion.String(),
		)
		return
	}

	if c.version.LT(MinSupportedVersion) {
		c.logger.Info(
			"Thanos version is below minimum supported version. Some features may not work correctly. Please upgrade to the minimum supported version or later.",
			"component", c.componentName,
			"resource", c.resourceName,
			"currentVersion", c.version.String(),
			"minSupportedVersion", MinSupportedVersion.String(),
		)
	}
}

// IsSupportedVersion returns true if the detected version is at or above the minimum supported version.
// Returns true if version is unknown (to allow custom images).
func (c *Checker) IsSupportedVersion() bool {
	if c.version == nil {
		return true // Allow custom images
	}
	return c.version.GTE(MinSupportedVersion)
}
