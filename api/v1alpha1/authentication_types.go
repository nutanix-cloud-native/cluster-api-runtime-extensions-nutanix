// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// AudienceMatchPolicy defines how the audiences field matches the aud claim.
type AudienceMatchPolicy string

const (
	// AudienceMatchPolicyMatchAny requires the aud claim to match at least one
	// entry in the audiences list.
	AudienceMatchPolicyMatchAny AudienceMatchPolicy = "MatchAny"
)

// Authentication configures API server JWT/OIDC authentication via
// the Structured Authentication Configuration file
// (apiserver.config.k8s.io/v1, requires Kubernetes >= 1.34).
// When set, CAREN renders an AuthenticationConfiguration file and sets
// --authentication-config. When unset the feature is a no-op for existing
// clusters.
// +kubebuilder:validation:XValidation:rule="self.jwt.all(a, self.jwt.exists_one(b, b.issuer.url == a.issuer.url))",message="issuer URLs must be unique across jwt authenticators"
// +kubebuilder:validation:XValidation:rule="self.jwt.all(a, !has(a.issuer.discoveryURL) || self.jwt.exists_one(b, has(b.issuer.discoveryURL) && b.issuer.discoveryURL == a.issuer.discoveryURL))",message="issuer discoveryURLs must be unique across jwt authenticators"
type Authentication struct {
	// JWT is the ordered list of JWT authenticators. Each entry must have a
	// unique issuer URL. Order defines the API server evaluation order.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=32
	JWT []JWTAuthenticator `json:"jwt"`
}

// JWTAuthenticator is one OIDC/JWT authenticator entry in the
// AuthenticationConfiguration file, mirroring apiserver.config.k8s.io/v1.
type JWTAuthenticator struct {
	// Issuer holds OIDC provider connection settings.
	// +kubebuilder:validation:Required
	Issuer Issuer `json:"issuer"`

	// ClaimValidationRules are applied after OIDC signature/issuer validation.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems=32
	ClaimValidationRules []ClaimValidationRule `json:"claimValidationRules,omitempty"`

	// ClaimMappings maps token claims to Kubernetes user info attributes.
	// +kubebuilder:validation:Required
	ClaimMappings ClaimMappings `json:"claimMappings"`

	// UserValidationRules are applied to the final user info before
	// completing authentication.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems=32
	UserValidationRules []UserValidationRule `json:"userValidationRules,omitempty"`
}

// Issuer holds OIDC provider connection settings for one JWT authenticator.
// +kubebuilder:validation:XValidation:rule="size(self.audiences) <= 1 || (has(self.audienceMatchPolicy) && self.audienceMatchPolicy == 'MatchAny')",message="audienceMatchPolicy must be MatchAny when more than one audience is set"
// +kubebuilder:validation:XValidation:rule="!has(self.discoveryURL) || self.discoveryURL.trim() != self.url.trim()",message="discoveryURL must differ from url"
type Issuer struct {
	// URL is the OIDC issuer URL (must be https:// without query or fragment).
	// Must match the iss claim in presented JWTs.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^https://[^/?#\s]+(/[^\s?#]*)?$`
	// +kubebuilder:validation:MinLength=9
	// +kubebuilder:validation:MaxLength=2048
	URL string `json:"url"`

	// DiscoveryURL overrides the OIDC discovery URL.
	// If set, discovery is fetched from this exact value instead of
	// {url}/.well-known/openid-configuration.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^https://[^/?#\s]+(/[^\s?#]*)?$`
	// +kubebuilder:validation:MinLength=9
	// +kubebuilder:validation:MaxLength=2048
	DiscoveryURL string `json:"discoveryURL,omitempty"`

	// CertificateAuthority contains PEM-encoded CA certificates for
	// validating the OIDC discovery connection.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=65536
	CertificateAuthority string `json:"certificateAuthority,omitempty"`

	// Audiences is the set of acceptable audiences the JWT must be issued to.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=32
	// +kubebuilder:validation:UniqueItems=true
	// +kubebuilder:validation:items:MinLength=1
	// +kubebuilder:validation:items:MaxLength=256
	Audiences []string `json:"audiences"`

	// AudienceMatchPolicy defines how audiences matches the aud claim.
	// Valid values are \"\" (default, single-audience semantics) or MatchAny.
	// MatchAny is required when more than one audience is set.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum="";MatchAny
	AudienceMatchPolicy AudienceMatchPolicy `json:"audienceMatchPolicy,omitempty"`
}

// ClaimMappings maps token claims to Kubernetes user info.
// Mirrors apiserver.config.k8s.io/v1 ClaimMappings.
type ClaimMappings struct {
	// Username maps a claim or CEL expression to the username.
	// +kubebuilder:validation:Required
	Username PrefixedClaimOrExpression `json:"username"`

	// Groups maps a claim or CEL expression to the groups.
	// +kubebuilder:validation:Optional
	Groups *PrefixedClaimOrExpression `json:"groups,omitempty"`

	// UID maps a claim or CEL expression to the UID.
	// +kubebuilder:validation:Optional
	UID *ClaimOrExpression `json:"uid,omitempty"`

	// Extra maps additional claims to extra user info keys.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems=32
	// +kubebuilder:validation:XValidation:rule="self.all(a, self.exists_one(b, b.key == a.key))",message="extra mapping keys must be unique"
	Extra []ExtraMapping `json:"extra,omitempty"`
}

// ClaimOrExpression selects a JWT claim or a CEL expression.
// +kubebuilder:validation:XValidation:rule="(has(self.claim) && !has(self.expression)) || (!has(self.claim) && has(self.expression))",message="set either claim or expression"
type ClaimOrExpression struct {
	// Claim is the JWT claim name to use.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	Claim string `json:"claim,omitempty"`

	// Expression is a CEL expression evaluated against token claims.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=4096
	Expression string `json:"expression,omitempty"`
}

// PrefixedClaimOrExpression selects a claim plus a prefix, or a CEL expression.
// +kubebuilder:validation:XValidation:rule="(has(self.expression) && !has(self.claim) && !has(self.prefix)) || (has(self.claim) && has(self.prefix) && !has(self.expression))",message="set either expression, or claim and prefix; prefix may be an empty string to disable prefixing"
type PrefixedClaimOrExpression struct {
	// Claim is the JWT claim name to use.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	Claim string `json:"claim,omitempty"`

	// Prefix is prepended to the claim value to prevent clashes with existing names.
	// Required when claim is set. An empty string disables prefixing.
	// "-" is a literal prefix. Mutually exclusive with expression.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=256
	Prefix *string `json:"prefix,omitempty"`

	// Expression is a CEL expression evaluated against token claims.
	// Mutually exclusive with claim and prefix.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=4096
	Expression string `json:"expression,omitempty"`
}

// ExtraMapping maps a CEL expression result to an extra user-info key.
type ExtraMapping struct {
	// Key is the extra attribute key (domain-prefix path, lowercase).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*\/[a-z0-9._-]+$`
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:XValidation:rule="!(self.split('/')[0] in ['k8s.io', 'kubernetes.io']) && !self.split('/')[0].endsWith('.k8s.io') && !self.split('/')[0].endsWith('.kubernetes.io')",message="k8s.io, kubernetes.io, and their subdomains are reserved"
	Key string `json:"key"`

	// ValueExpression is a CEL expression whose result populates the extra value.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=4096
	ValueExpression string `json:"valueExpression"`
}

// ClaimValidationRule is one validation rule for token claims.
// +kubebuilder:validation:XValidation:rule="(has(self.claim) && has(self.requiredValue) && !has(self.expression) && !has(self.message)) || (!has(self.claim) && !has(self.requiredValue) && has(self.expression))",message="claimValidationRules entry must be either {claim + requiredValue} or {expression + optional message}"
type ClaimValidationRule struct {
	// Claim is the required claim name.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	Claim string `json:"claim,omitempty"`

	// RequiredValue is the required value for the claim.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1024
	RequiredValue string `json:"requiredValue,omitempty"`

	// Expression is a CEL validation expression.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=4096
	Expression string `json:"expression,omitempty"`

	// Message customizes the error when expression returns false.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1024
	Message string `json:"message,omitempty"`
}

// UserValidationRule validates final user info via CEL.
type UserValidationRule struct {
	// Expression is a CEL expression evaluated against the user object.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=4096
	Expression string `json:"expression"`

	// Message customizes the error when expression returns false.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1024
	Message string `json:"message,omitempty"`
}
