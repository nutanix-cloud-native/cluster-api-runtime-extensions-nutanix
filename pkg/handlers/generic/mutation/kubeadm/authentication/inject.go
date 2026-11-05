// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

const (
	// VariableName is the external patch variable name.
	VariableName = "authentication"

	// AuthenticationConfigPath is the well-known path for the
	// AuthenticationConfiguration file on control plane nodes.
	AuthenticationConfigPath = "/etc/kubernetes/authentication.yaml"

	// AuthenticationConfigArg is the API server flag name.
	AuthenticationConfigArg = "authentication-config"

	authenticationConfigVolumeName = "authentication-config"
)

type authPatchHandler struct {
	variableName      string
	variableFieldPath []string
}

// NewPatch creates a new authentication patch handler.
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

	log = log.WithValues(
		"variableName", h.variableName,
		"variableFieldPath", h.variableFieldPath,
		"variableValue", authVar,
	)

	return patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			// Nil or empty -> cleanup (idempotent removal for reconciliation after feature removal).
			if authVar == nil || len(authVar.JWT) == 0 {
				log.V(5).Info("authentication not specified, ensuring removal")
				spec := &obj.Spec.Template.Spec.KubeadmConfigSpec
				spec.Files = removeFile(spec.Files, AuthenticationConfigPath)
				apiServer := &spec.ClusterConfiguration.APIServer
				apiServer.ExtraVolumes = removeVolumeMount(apiServer.ExtraVolumes, AuthenticationConfigPath, authenticationConfigVolumeName)
				apiServer.ExtraArgs = removeArg(apiServer.ExtraArgs, AuthenticationConfigArg)
				return nil
			}

			apiServer := &obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer

			// Mutually exclusive with legacy --oidc-* flags.
			for _, arg := range apiServer.ExtraArgs {
				if strings.HasPrefix(arg.Name, "oidc-") {
					return fmt.Errorf(
						"use of --oidc-* flags is mutually exclusive with authentication; remove the --oidc-* extraArgs",
					)
				}
			}

			// Conflict check: existing --authentication-config with different path.
			for _, arg := range apiServer.ExtraArgs {
				if arg.Name == AuthenticationConfigArg && arg.Value != nil && *arg.Value != AuthenticationConfigPath {
					return fmt.Errorf(
						"existing --%s flag value %q conflicts with well-known path %q",
						AuthenticationConfigArg, *arg.Value, AuthenticationConfigPath,
					)
				}
			}

			content, err := renderAuthenticationConfig(authVar)
			if err != nil {
				return fmt.Errorf("failed to render authentication config: %w", err)
			}

			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("adding authentication configuration")

			spec := &obj.Spec.Template.Spec.KubeadmConfigSpec
			spec.Files = ensureFile(spec.Files, AuthenticationConfigPath, content)
			apiServer.ExtraVolumes = ensureVolumeMount(apiServer.ExtraVolumes, AuthenticationConfigPath, authenticationConfigVolumeName)
			apiServer.ExtraArgs = ensureArg(apiServer.ExtraArgs, AuthenticationConfigArg, AuthenticationConfigPath)

			return nil
		},
	)
}

// render types - local structs mirroring apiserver.config.k8s.io/v1 AuthenticationConfiguration.

type authnConfig struct {
	APIVersion string             `json:"apiVersion"`
	Kind       string             `json:"kind"`
	JWT        []jwtAuthenticator `json:"jwt"`
}

type jwtAuthenticator struct {
	Issuer               issuer                `json:"issuer"`
	ClaimValidationRules []claimValidationRule `json:"claimValidationRules,omitempty"`
	ClaimMappings        claimMappings         `json:"claimMappings"`
	UserValidationRules  []userValidationRule  `json:"userValidationRules,omitempty"`
}

type issuer struct {
	URL                  string   `json:"url"`
	DiscoveryURL         string   `json:"discoveryURL,omitempty"`
	CertificateAuthority string   `json:"certificateAuthority,omitempty"`
	Audiences            []string `json:"audiences"`
	AudienceMatchPolicy  string   `json:"audienceMatchPolicy,omitempty"`
}

type claimMappings struct {
	Username prefixedClaimOrExpression  `json:"username"`
	Groups   *prefixedClaimOrExpression `json:"groups,omitempty"`
	UID      *claimOrExpression         `json:"uid,omitempty"`
	Extra    []extraMapping             `json:"extra,omitempty"`
}

type claimOrExpression struct {
	Claim      string `json:"claim,omitempty"`
	Expression string `json:"expression,omitempty"`
}

type prefixedClaimOrExpression struct {
	Claim      string  `json:"claim,omitempty"`
	Prefix     *string `json:"prefix,omitempty"`
	Expression string  `json:"expression,omitempty"`
}

type extraMapping struct {
	Key             string `json:"key"`
	ValueExpression string `json:"valueExpression"`
}

type claimValidationRule struct {
	Claim         string `json:"claim,omitempty"`
	RequiredValue string `json:"requiredValue,omitempty"`
	Expression    string `json:"expression,omitempty"`
	Message       string `json:"message,omitempty"`
}

type userValidationRule struct {
	Expression string `json:"expression"`
	Message    string `json:"message,omitempty"`
}

func renderAuthenticationConfig(auth *v1alpha1.Authentication) (string, error) {
	cfg := authnConfig{
		APIVersion: "apiserver.config.k8s.io/v1",
		Kind:       "AuthenticationConfiguration",
		JWT:        make([]jwtAuthenticator, 0, len(auth.JWT)),
	}
	for _, j := range auth.JWT {
		r := jwtAuthenticator{
			Issuer: issuer{
				URL:                  j.Issuer.URL,
				DiscoveryURL:         j.Issuer.DiscoveryURL,
				CertificateAuthority: j.Issuer.CertificateAuthority,
				Audiences:            j.Issuer.Audiences,
			},
			ClaimMappings: claimMappings{
				Username: prefixedClaimOrExpression{
					Claim:      j.ClaimMappings.Username.Claim,
					Prefix:     j.ClaimMappings.Username.Prefix,
					Expression: j.ClaimMappings.Username.Expression,
				},
			},
		}
		if j.Issuer.AudienceMatchPolicy != "" {
			r.Issuer.AudienceMatchPolicy = string(j.Issuer.AudienceMatchPolicy)
		}
		// ClaimValidationRules - preserve order
		if len(j.ClaimValidationRules) > 0 {
			r.ClaimValidationRules = make([]claimValidationRule, 0, len(j.ClaimValidationRules))
			for _, c := range j.ClaimValidationRules {
				r.ClaimValidationRules = append(r.ClaimValidationRules, claimValidationRule{
					Claim:         c.Claim,
					RequiredValue: c.RequiredValue,
					Expression:    c.Expression,
					Message:       c.Message,
				})
			}
		}
		// ClaimMappings optional fields
		if j.ClaimMappings.Groups != nil {
			r.ClaimMappings.Groups = &prefixedClaimOrExpression{
				Claim:      j.ClaimMappings.Groups.Claim,
				Prefix:     j.ClaimMappings.Groups.Prefix,
				Expression: j.ClaimMappings.Groups.Expression,
			}
		}
		if j.ClaimMappings.UID != nil {
			r.ClaimMappings.UID = &claimOrExpression{
				Claim:      j.ClaimMappings.UID.Claim,
				Expression: j.ClaimMappings.UID.Expression,
			}
		}
		if len(j.ClaimMappings.Extra) > 0 {
			r.ClaimMappings.Extra = make([]extraMapping, 0, len(j.ClaimMappings.Extra))
			for _, e := range j.ClaimMappings.Extra {
				r.ClaimMappings.Extra = append(r.ClaimMappings.Extra, extraMapping{
					Key:             e.Key,
					ValueExpression: e.ValueExpression,
				})
			}
		}
		if len(j.UserValidationRules) > 0 {
			r.UserValidationRules = make([]userValidationRule, 0, len(j.UserValidationRules))
			for _, u := range j.UserValidationRules {
				r.UserValidationRules = append(r.UserValidationRules, userValidationRule{
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
	return string(out), nil
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

func removeFile(files []bootstrapv1.File, path string) []bootstrapv1.File {
	out := files[:0]
	for _, f := range files {
		if f.Path == path {
			continue
		}
		out = append(out, f)
	}
	return out
}

func ensureVolumeMount(volumes []bootstrapv1.HostPathMount, path, name string) []bootstrapv1.HostPathMount {
	for _, v := range volumes {
		if v.MountPath == path {
			return volumes
		}
	}
	return append(volumes, bootstrapv1.HostPathMount{
		Name:      name,
		HostPath:  path,
		MountPath: path,
		ReadOnly:  new(true),
		PathType:  corev1.HostPathFile,
	})
}

func removeVolumeMount(volumes []bootstrapv1.HostPathMount, path, name string) []bootstrapv1.HostPathMount {
	out := volumes[:0]
	for _, v := range volumes {
		if v.Name == name || v.MountPath == path || v.HostPath == path {
			continue
		}
		out = append(out, v)
	}
	return out
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

func removeArg(args []bootstrapv1.Arg, name string) []bootstrapv1.Arg {
	out := args[:0]
	for _, a := range args {
		if a.Name == name {
			continue
		}
		out = append(out, a)
	}
	return out
}
