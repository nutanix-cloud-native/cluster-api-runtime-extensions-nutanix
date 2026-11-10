// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// ServiceAccountIssuer configures the API server's ServiceAccount token issuer.
// When nil on KubeadmClusterConfigSpec, no issuer flags are set (no-op).
// Maps to --service-account-issuer, --api-audiences, --service-account-jwks-uri.
type ServiceAccountIssuer struct {
	// Issuer is the identifier asserted in ServiceAccount JWT iss claims.
	// Must be an https:// URL. Triggers --service-account-issuer.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Format=uri
	// +kubebuilder:validation:Pattern=`^https://`
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2048
	Issuer string `json:"issuer"`

	// Audiences is an optional list of audiences tokens are bound to.
	// When set, rendered as comma-joined --api-audiences in declared order.
	// When absent, --api-audiences is not set (API server defaults to issuer URL).
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems=50
	// +kubebuilder:validation:UniqueItems=true
	// +kubebuilder:validation:items:MinLength=1
	// +kubebuilder:validation:items:MaxLength=253
	Audiences []string `json:"audiences,omitempty"`

	// JWKSURI overrides the JWKS URI advertised in the OIDC discovery document.
	// When set, must be an https:// URL. Triggers --service-account-jwks-uri.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Format=uri
	// +kubebuilder:validation:Pattern=`^https://`
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2048
	JWKSURI string `json:"jwksURI,omitempty"`
}
