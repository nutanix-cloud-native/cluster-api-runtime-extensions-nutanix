// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"cmp"
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/blang/semver/v4"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	apiserver "k8s.io/apiserver/pkg/apis/apiserver"
	apiserverinstall "k8s.io/apiserver/pkg/apis/apiserver/install"
	apiserverv1 "k8s.io/apiserver/pkg/apis/apiserver/v1"
	apiservervalidation "k8s.io/apiserver/pkg/apis/apiserver/validation"
	authenticationcel "k8s.io/apiserver/pkg/authentication/cel"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

var (
	versionGreaterOrEqualTo134Range = semver.MustParseRange(">=1.34.0-0")

	apiserverValidator = sync.OnceValues(func() (runtime.Decoder, authenticationcel.Compiler) {
		scheme := runtime.NewScheme()
		apiserverinstall.Install(scheme)
		codecs := serializer.NewCodecFactory(scheme, serializer.EnableStrict)
		return codecs.UniversalDecoder(), authenticationcel.NewDefaultCompiler()
	})
)

const (
	// VariableName is the external patch variable name.
	VariableName = "authentication"

	// AuthenticationConfigPath is the path for the
	// AuthenticationConfiguration file on control plane nodes.
	AuthenticationConfigPath = "/etc/kubernetes/authentication.yaml"

	// AuthenticationConfigArg is the API server flag name.
	AuthenticationConfigArg = "authentication-config"

	// ServiceAccountIssuerArg is the API server flag name for service account token issuer(s).
	ServiceAccountIssuerArg = "service-account-issuer"

	authenticationConfigVolumeName = "authentication-config"
	defaultServiceDomain           = "cluster.local"
)

type authPatchHandler struct {
	variableName      string
	variableFieldPath []string
}

// NewPatch creates a new authentication patch handler.
//
// It renders apiserver.config.k8s.io/v1 AuthenticationConfiguration
// (requires Kubernetes >= 1.34) when clusterConfig.authentication is set.
func NewPatch() *authPatchHandler {
	return newAuthPatchHandler(v1alpha1.ClusterConfigVariableName, VariableName)
}

func newAuthPatchHandler(variableName string, variableFieldPath ...string) *authPatchHandler {
	return &authPatchHandler{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

// Mutate implements the mutation handler for authentication configuration.
func (h *authPatchHandler) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ client.ObjectKey,
	_ mutation.ClusterGetter,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues("holderRef", holderRef)

	authVar, err := variables.Get[*v1alpha1.Authentication](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("authentication variable not defined")
			return nil
		}
		return err
	}
	if authVar == nil || len(authVar.JWT) == 0 {
		log.V(5).Info("authentication not specified, skipping mutation")
		return nil
	}

	log = log.WithValues(
		"variableName", h.variableName,
		"variableFieldPath", h.variableFieldPath,
		"variableValue", sanitizeAuthenticationForLog(authVar),
	)

	return patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			cpVersion, err := variables.Get[string](vars, runtimehooksv1.BuiltinsName, "controlPlane", "version")
			if err != nil {
				return fmt.Errorf("failed to get control plane Kubernetes version from builtin variable: %w", err)
			}
			if err := ValidateKubernetesVersion(cpVersion); err != nil {
				return err
			}

			serviceDomain, err := variables.Get[string](
				vars,
				runtimehooksv1.BuiltinsName,
				"cluster",
				"network",
				"serviceDomain",
			)
			if err != nil && !variables.IsNotFoundError(err) {
				return fmt.Errorf("failed to get cluster serviceDomain from builtin variable: %w", err)
			}

			disallowedIssuers, err := ValidateControlPlaneTemplate(obj, serviceDomain)
			if err != nil {
				return err
			}

			apiServer := &obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer
			extraVolumes, err := ensureVolumeMount(
				apiServer.ExtraVolumes,
				AuthenticationConfigPath,
				authenticationConfigVolumeName,
			)
			if err != nil {
				return err
			}

			content, err := renderAuthenticationConfig(authVar, disallowedIssuers)
			if err != nil {
				return fmt.Errorf("failed to render authentication config: %w", err)
			}

			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("adding authentication configuration")

			spec := &obj.Spec.Template.Spec.KubeadmConfigSpec
			spec.Files = ensureFile(spec.Files, AuthenticationConfigPath, content)
			apiServer.ExtraVolumes = extraVolumes
			apiServer.ExtraArgs = ensureArg(apiServer.ExtraArgs, AuthenticationConfigArg, AuthenticationConfigPath)

			return nil
		},
	)
}

// ValidateKubernetesVersion verifies that cpVersion is >= 1.34.0, which is required
// for apiserver.config.k8s.io/v1 AuthenticationConfiguration.
func ValidateKubernetesVersion(cpVersion string) error {
	kubernetesVersion, err := semver.ParseTolerant(cpVersion)
	if err != nil {
		return fmt.Errorf("failed to parse control plane Kubernetes version %q: %w", cpVersion, err)
	}
	if !versionGreaterOrEqualTo134Range(kubernetesVersion) {
		return fmt.Errorf(
			"authentication requires Kubernetes >= 1.34.0 (apiserver.config.k8s.io/v1), got %q",
			cpVersion,
		)
	}
	return nil
}

// DefaultServiceAccountIssuers returns kubeadm's default --service-account-issuer
// for the given cluster serviceDomain (defaulting to "cluster.local" when empty).
func DefaultServiceAccountIssuers(serviceDomain string) []string {
	domain := cmp.Or(strings.TrimPrefix(serviceDomain, "."), defaultServiceDomain)
	return []string{"https://kubernetes.default.svc." + domain}
}

// ValidateControlPlaneTemplate checks a KubeadmControlPlaneTemplate for conflicting
// --oidc-* flags, --authentication-config flags, and extraVolumes entries, and returns
// the effective --service-account-issuer values that JWT issuer URLs must not overlap.
func ValidateControlPlaneTemplate(
	obj *controlplanev1.KubeadmControlPlaneTemplate,
	serviceDomain string,
) ([]string, error) {
	apiServer := &obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer

	var serviceAccountIssuers []string
	for _, arg := range apiServer.ExtraArgs {
		if strings.HasPrefix(arg.Name, "oidc-") {
			return nil, fmt.Errorf(
				"use of --oidc-* flags is mutually exclusive with authentication; remove the --oidc-* extraArgs",
			)
		}
		if arg.Name == AuthenticationConfigArg {
			if arg.Value == nil {
				return nil, fmt.Errorf(
					"existing --%s flag has nil value; expected %q",
					AuthenticationConfigArg, AuthenticationConfigPath,
				)
			}
			if *arg.Value != AuthenticationConfigPath {
				return nil, fmt.Errorf(
					"existing --%s flag value %q does not match expected path %q",
					AuthenticationConfigArg, *arg.Value, AuthenticationConfigPath,
				)
			}
		}
		if arg.Name == ServiceAccountIssuerArg && arg.Value != nil && *arg.Value != "" {
			serviceAccountIssuers = append(serviceAccountIssuers, *arg.Value)
		}
	}

	if _, err := ensureVolumeMount(
		apiServer.ExtraVolumes,
		AuthenticationConfigPath,
		authenticationConfigVolumeName,
	); err != nil {
		return nil, err
	}

	if len(serviceAccountIssuers) == 0 {
		serviceAccountIssuers = DefaultServiceAccountIssuers(serviceDomain)
	}
	return serviceAccountIssuers, nil
}

// ValidateConfiguration renders auth to apiserver.config.k8s.io/v1 AuthenticationConfiguration
// and validates it with upstream kube-apiserver validation rules, including CEL compilation
// and disallowed service-account issuer overlap checks.
func ValidateConfiguration(auth *v1alpha1.Authentication, disallowedIssuers []string) error {
	_, err := renderAuthenticationConfig(auth, disallowedIssuers)
	return err
}

func renderAuthenticationConfig(auth *v1alpha1.Authentication, disallowedIssuers []string) (string, error) {
	cfg := apiserverv1.AuthenticationConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiserver.config.k8s.io/v1",
			Kind:       "AuthenticationConfiguration",
		},
		JWT: make([]apiserverv1.JWTAuthenticator, 0, len(auth.JWT)),
	}
	for i := range auth.JWT {
		j := auth.JWT[i]
		r := apiserverv1.JWTAuthenticator{
			Issuer: apiserverv1.Issuer{
				URL:                  j.Issuer.URL,
				DiscoveryURL:         stringPtrOrNil(j.Issuer.DiscoveryURL),
				CertificateAuthority: j.Issuer.CertificateAuthority,
				Audiences:            j.Issuer.Audiences,
			},
			ClaimMappings: apiserverv1.ClaimMappings{
				Username: apiserverv1.PrefixedClaimOrExpression{
					Claim:      j.ClaimMappings.Username.Claim,
					Prefix:     j.ClaimMappings.Username.Prefix,
					Expression: j.ClaimMappings.Username.Expression,
				},
			},
		}
		if j.Issuer.AudienceMatchPolicy != "" {
			r.Issuer.AudienceMatchPolicy = apiserverv1.AudienceMatchPolicyType(j.Issuer.AudienceMatchPolicy)
		}
		if len(j.ClaimValidationRules) > 0 {
			r.ClaimValidationRules = make([]apiserverv1.ClaimValidationRule, 0, len(j.ClaimValidationRules))
			for _, c := range j.ClaimValidationRules {
				r.ClaimValidationRules = append(r.ClaimValidationRules, apiserverv1.ClaimValidationRule{
					Claim:         c.Claim,
					RequiredValue: c.RequiredValue,
					Expression:    c.Expression,
					Message:       c.Message,
				})
			}
		}
		if j.ClaimMappings.Groups != nil {
			r.ClaimMappings.Groups = apiserverv1.PrefixedClaimOrExpression{
				Claim:      j.ClaimMappings.Groups.Claim,
				Prefix:     j.ClaimMappings.Groups.Prefix,
				Expression: j.ClaimMappings.Groups.Expression,
			}
		}
		if j.ClaimMappings.UID != nil {
			r.ClaimMappings.UID = apiserverv1.ClaimOrExpression{
				Claim:      j.ClaimMappings.UID.Claim,
				Expression: j.ClaimMappings.UID.Expression,
			}
		}
		if len(j.ClaimMappings.Extra) > 0 {
			r.ClaimMappings.Extra = make([]apiserverv1.ExtraMapping, 0, len(j.ClaimMappings.Extra))
			for _, e := range j.ClaimMappings.Extra {
				r.ClaimMappings.Extra = append(r.ClaimMappings.Extra, apiserverv1.ExtraMapping{
					Key:             e.Key,
					ValueExpression: e.ValueExpression,
				})
			}
		}
		if len(j.UserValidationRules) > 0 {
			r.UserValidationRules = make([]apiserverv1.UserValidationRule, 0, len(j.UserValidationRules))
			for _, u := range j.UserValidationRules {
				r.UserValidationRules = append(r.UserValidationRules, apiserverv1.UserValidationRule{
					Expression: u.Expression,
					Message:    u.Message,
				})
			}
		}
		cfg.JWT = append(cfg.JWT, r)
	}
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	if err := validateRenderedAuthenticationConfig(out, disallowedIssuers); err != nil {
		return "", err
	}
	return string(out), nil
}

func validateRenderedAuthenticationConfig(raw []byte, disallowedIssuers []string) error {
	decoder, compiler := apiserverValidator()
	obj, err := runtime.Decode(decoder, raw)
	if err != nil {
		return fmt.Errorf("decoding rendered AuthenticationConfiguration: %w", err)
	}
	internalCfg, ok := obj.(*apiserver.AuthenticationConfiguration)
	if !ok {
		return fmt.Errorf("unexpected decoded type %T", obj)
	}
	if errs := apiservervalidation.ValidateAuthenticationConfiguration(
		compiler,
		internalCfg,
		disallowedIssuers,
	); len(errs) > 0 {
		return errs.ToAggregate()
	}
	return nil
}

func stringPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

const redactedCertificateAuthority = "<omitted>"

func sanitizeAuthenticationForLog(auth *v1alpha1.Authentication) *v1alpha1.Authentication {
	if auth == nil {
		return nil
	}
	copied := auth.DeepCopy()
	for i := range copied.JWT {
		if copied.JWT[i].Issuer.CertificateAuthority != "" {
			copied.JWT[i].Issuer.CertificateAuthority = redactedCertificateAuthority
		}
	}
	return copied
}

func ensureFile(files []bootstrapv1.File, path, content string) []bootstrapv1.File {
	for i := range files {
		if files[i].Path == path {
			files[i].Content = content
			files[i].Permissions = "0600"
			return files
		}
	}
	return append(files, bootstrapv1.File{
		Path:        path,
		Permissions: "0600",
		Content:     content,
	})
}

func ensureVolumeMount(
	volumes []bootstrapv1.HostPathMount,
	path, name string,
) ([]bootstrapv1.HostPathMount, error) {
	for _, v := range volumes {
		if v.Name == name || v.MountPath == path || v.HostPath == path {
			if v.Name != name || v.MountPath != path || v.HostPath != path {
				return nil, fmt.Errorf(
					"existing apiServer extraVolume %q (%s -> %s) conflicts with %q (%s -> %s)",
					v.Name, v.HostPath, v.MountPath, name, path, path,
				)
			}
			return volumes, nil
		}
	}
	return append(volumes, bootstrapv1.HostPathMount{
		Name:      name,
		HostPath:  path,
		MountPath: path,
		ReadOnly:  new(true),
		PathType:  corev1.HostPathFile,
	}), nil
}

func ensureArg(args []bootstrapv1.Arg, name, value string) []bootstrapv1.Arg {
	for i := range args {
		if args[i].Name == name {
			// Already correct.
			if args[i].Value != nil && *args[i].Value == value {
				return args
			}
			args[i].Value = &value
			return args
		}
	}
	return append(args, bootstrapv1.Arg{
		Name:  name,
		Value: &value,
	})
}
