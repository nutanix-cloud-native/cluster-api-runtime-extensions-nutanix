// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiserverv1 "k8s.io/apiserver/pkg/apis/apiserver/v1"
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

	// AuthenticationConfigPath is the path for the
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
	if authVar == nil || len(authVar.JWT) == 0 {
		log.V(5).Info("authentication not specified, skipping mutation")
		return nil
	}

	log = log.WithValues(
		"variableName", h.variableName,
		"variableFieldPath", h.variableFieldPath,
		"variableValue", authVar,
	)

	return patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			apiServer := &obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer

			// Mutually exclusive with legacy --oidc-* flags.
			for _, arg := range apiServer.ExtraArgs {
				if strings.HasPrefix(arg.Name, "oidc-") {
					return fmt.Errorf(
						"use of --oidc-* flags is mutually exclusive with authentication; remove the --oidc-* extraArgs",
					)
				}
			}

			// Conflict check: existing --authentication-config with unexpected path.
			for _, arg := range apiServer.ExtraArgs {
				if arg.Name == AuthenticationConfigArg && arg.Value != nil && *arg.Value != AuthenticationConfigPath {
					return fmt.Errorf(
						"existing --%s flag value %q does not match expected path %q",
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
			apiServer.ExtraVolumes = ensureVolumeMount(
				apiServer.ExtraVolumes,
				AuthenticationConfigPath,
				authenticationConfigVolumeName,
			)
			apiServer.ExtraArgs = ensureArg(apiServer.ExtraArgs, AuthenticationConfigArg, AuthenticationConfigPath)

			return nil
		},
	)
}

func renderAuthenticationConfig(auth *v1alpha1.Authentication) (string, error) {
	cfg := apiserverv1.AuthenticationConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiserver.config.k8s.io/v1",
			Kind:       "AuthenticationConfiguration",
		},
		JWT: make([]apiserverv1.JWTAuthenticator, 0, len(auth.JWT)),
	}
	for _, j := range auth.JWT {
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
	return string(out), nil
}


func stringPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
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
