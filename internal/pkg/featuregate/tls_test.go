package featuregate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServerTLSConfigFile(t *testing.T) {
	for _, tc := range []struct {
		name, yaml                    string
		enabled, wantError, automatic bool
	}{
		{"defaults", "", true, false, true},
		{"issuer", `server-tls:
  certManager:
    issuerRef: {name: platform, kind: ClusterIssuer}
    caBundleConfigMap: {name: trust}
`, true, false, false},
		{"issuer without trust", `server-tls:
  certManager:
    issuerRef: {name: platform}
`, true, true, false},
		{"trust without issuer", `server-tls:
  certManager:
    caBundleConfigMap: {name: trust}
`, true, true, false},
		{"unsupported provider", "server-tls: {provider: other}", true, true, false},
		{"invalid kind", `server-tls:
  certManager:
    issuerRef: {name: platform, kind: Secret}
    caBundleConfigMap: {name: trust}
`, true, true, false},
		{"disabled ignores settings", "server-tls: [invalid]", false, false, false},
		{"yaml cannot enable", "server-tls: {enabled: true}", false, false, false},
		{"yaml cannot disable", "server-tls: {enabled: false}", true, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "features.yaml")
			require.NoError(t, os.WriteFile(path, []byte(tc.yaml), 0600))
			var flags Flag
			if tc.enabled {
				require.NoError(t, flags.Set(ServerTLS))
			}
			cfg, err := LoadAndApplyConfig(path, flags.ToFeatureGate())
			if tc.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.enabled, cfg.ServerTLSEnabled())
			if !tc.enabled {
				return
			}
			require.Equal(t, CertManagerProvider, cfg.ServerTLS.Provider)
			require.Equal(t, tc.automatic, cfg.ServerTLS.Automatic())
			require.Equal(t, TLSCAKey, cfg.ServerTLS.CABundle().Key)
		})
	}
}
