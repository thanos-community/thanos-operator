package featuregate

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	CertManagerProvider = "cert-manager"
	TLSCAName           = "thanos-operator-ca"
	TLSCAKey            = "ca.crt"
)

type TLSConfig struct {
	FeatureConfig `json:"-"`
	Provider      string             `json:"provider,omitempty"`
	CertManager   *CertManagerConfig `json:"certManager,omitempty"`
}

type CertManagerConfig struct {
	IssuerRef         *IssuerReference   `json:"issuerRef,omitempty"`
	CABundleConfigMap *CABundleReference `json:"caBundleConfigMap,omitempty"`
}

type IssuerReference struct {
	Name  string `json:"name"`
	Kind  string `json:"kind,omitempty"`
	Group string `json:"group,omitempty"`
}

type CABundleReference struct {
	Name string `json:"name"`
	Key  string `json:"key,omitempty"`
}

func (c TLSConfig) Automatic() bool {
	return c.CertManager == nil || (c.CertManager.IssuerRef == nil && c.CertManager.CABundleConfigMap == nil)
}

func (c TLSConfig) Issuer() IssuerReference {
	ref := IssuerReference{Name: TLSCAName, Kind: "Issuer", Group: "cert-manager.io"}
	if c.CertManager != nil && c.CertManager.IssuerRef != nil {
		ref = *c.CertManager.IssuerRef
		if ref.Kind == "" {
			ref.Kind = "Issuer"
		}
		if ref.Group == "" {
			ref.Group = "cert-manager.io"
		}
	}
	return ref
}

func (c TLSConfig) CABundle() CABundleReference {
	ref := CABundleReference{Name: TLSCAName, Key: TLSCAKey}
	if c.CertManager != nil && c.CertManager.CABundleConfigMap != nil {
		ref = *c.CertManager.CABundleConfigMap
		if ref.Key == "" {
			ref.Key = TLSCAKey
		}
	}
	return ref
}

func (c TLSConfig) Validate() error {
	if c.Provider != "" && c.Provider != CertManagerProvider {
		return fmt.Errorf("unsupported TLS provider %q", c.Provider)
	}
	if c.Automatic() {
		return nil
	}
	if c.CertManager.IssuerRef == nil || c.CertManager.CABundleConfigMap == nil {
		return fmt.Errorf("certManager requires both issuerRef and caBundleConfigMap")
	}
	issuer, ca := c.Issuer(), c.CABundle()
	if errs := validation.IsDNS1123Subdomain(issuer.Name); len(errs) != 0 {
		return fmt.Errorf("invalid issuerRef.name %q: %v", issuer.Name, errs)
	}
	if issuer.Kind != "Issuer" && issuer.Kind != "ClusterIssuer" {
		return fmt.Errorf("issuerRef.kind must be Issuer or ClusterIssuer")
	}
	if issuer.Group != "cert-manager.io" {
		return fmt.Errorf("issuerRef.group must be cert-manager.io")
	}
	if errs := validation.IsDNS1123Subdomain(ca.Name); len(errs) != 0 {
		return fmt.Errorf("invalid caBundleConfigMap.name %q: %v", ca.Name, errs)
	}
	if errs := validation.IsConfigMapKey(ca.Key); len(errs) != 0 {
		return fmt.Errorf("invalid caBundleConfigMap.key %q: %v", ca.Key, errs)
	}
	return nil
}
