// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceAccountIssuer configures the API server's ServiceAccount token issuer.
// When nil on KubeadmClusterConfigSpec, no issuer flags are set (no-op).
// Maps to --service-account-issuer, --api-audiences, --service-account-jwks-uri,
// --service-account-max-token-expiration.
type ServiceAccountIssuer struct {
	// Issuer is the identifier asserted in ServiceAccount JWT iss claims.
	// Must be an https:// URL with a non-empty hostname and without userinfo, query, or fragment.
	// Triggers --service-account-issuer.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Format=uri
	// +kubebuilder:validation:Pattern=`^https://[^/?#@\s]+(/[^\s?#]*)?$`
	// +kubebuilder:validation:XValidation:rule="isURL(self) && url(self).getHostname() != '' && (url(self).getPort() == '' || (int(url(self).getPort()) >= 1 && int(url(self).getPort()) <= 65535))",message="must be a valid https URL with a non-empty hostname and valid port"
	// +kubebuilder:validation:MinLength=9
	// +kubebuilder:validation:MaxLength=2048
	Issuer string `json:"issuer"`

	// Audiences is an optional list of audiences tokens are bound to.
	// Entries must not contain whitespace, commas, or double quotes because
	// kube-apiserver parses --api-audiences as CSV via pflag.StringSlice.
	// When set, rendered as comma-joined --api-audiences sorted alphabetically.
	// When absent, --api-audiences is not set (API server defaults to issuer URL).
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems=50
	// +kubebuilder:validation:UniqueItems=true
	// +kubebuilder:validation:items:MinLength=1
	// +kubebuilder:validation:items:MaxLength=253
	// +kubebuilder:validation:items:Pattern=`^[^\s,"]+$`
	Audiences []string `json:"audiences,omitempty"`

	// JWKSURI overrides the JWKS URI advertised in the OIDC discovery document.
	// When set, must be an https:// URL with a non-empty hostname and without userinfo or fragment
	// (query parameters are permitted per OpenID Connect Discovery 1.0 Section 3).
	// Triggers --service-account-jwks-uri.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Format=uri
	// +kubebuilder:validation:Pattern=`^https://[^/?#@\s]+(/[^\s?#]*)?(\?[^\s#]*)?$`
	// +kubebuilder:validation:XValidation:rule="isURL(self) && url(self).getHostname() != '' && (url(self).getPort() == '' || (int(url(self).getPort()) >= 1 && int(url(self).getPort()) <= 65535))",message="must be a valid https URL with a non-empty hostname and valid port"
	// +kubebuilder:validation:MinLength=9
	// +kubebuilder:validation:MaxLength=2048
	JWKSURI string `json:"jwksURI,omitempty"`

	// MaxTokenExpiration is the maximum lifetime for ServiceAccount tokens.
	// Must be between 1h and 1193046h28m16s (2^32 seconds), matching kube-apiserver validation.
	// Note that kube-apiserver enables --service-account-extend-token-expiration=true by default,
	// which extends default admission-injected Pod projected tokens up to 1 year unless the Pod's
	// serviceAccountToken projection sets expirationSeconds explicitly.
	// When set, triggers --service-account-max-token-expiration.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:XValidation:rule="duration(self) >= duration('1h') && duration(self) <= duration('1193046h28m16s')",message="must be between 1h and 1193046h28m16s (2^32s)"
	MaxTokenExpiration *metav1.Duration `json:"maxTokenExpiration,omitempty"`
}
