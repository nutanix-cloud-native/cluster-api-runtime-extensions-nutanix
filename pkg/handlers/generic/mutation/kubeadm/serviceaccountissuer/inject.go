// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package serviceaccountissuer

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/pflag"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/util/webhook"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "serviceAccountIssuer"

	// ExtraArgs names for API server.
	ServiceAccountIssuerArg      = "service-account-issuer"
	APIAudiencesArg              = "api-audiences"
	ServiceAccountJWKSURIArg     = "service-account-jwks-uri"
	ServiceAccountMaxTokenExpArg = "service-account-max-token-expiration"

	// Bounds enforced by kube-apiserver's ValidateServiceAccountTokenFlags.
	minServiceAccountTokenExpiration = time.Hour
	maxServiceAccountTokenExpiration = (1 << 32) * time.Second
)

type serviceAccountIssuerPatchHandler struct {
	variableName      string
	variableFieldPath []string
}

func NewPatch() *serviceAccountIssuerPatchHandler {
	return newServiceAccountIssuerPatchHandler(v1alpha1.ClusterConfigVariableName, VariableName)
}

func newServiceAccountIssuerPatchHandler(
	variableName string,
	variableFieldPath ...string,
) *serviceAccountIssuerPatchHandler {
	return &serviceAccountIssuerPatchHandler{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *serviceAccountIssuerPatchHandler) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ client.ObjectKey,
	_ mutation.ClusterGetter,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	sai, err := variables.Get[*v1alpha1.ServiceAccountIssuer](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("ServiceAccount issuer variable not defined")
			return nil
		}
		return err
	}

	if sai == nil {
		log.V(5).Info("ServiceAccount issuer not specified, skipping mutation")
		return nil
	}

	log = log.WithValues(
		"variableName", h.variableName,
		"variableFieldPath", h.variableFieldPath,
		"variableValue", sai,
	)

	return patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			var authCfg *v1alpha1.Authentication
			if len(h.variableFieldPath) > 0 {
				authPath := append(
					slices.Clone(h.variableFieldPath[:len(h.variableFieldPath)-1]),
					"authentication",
				)
				loadedAuth, authErr := variables.Get[*v1alpha1.Authentication](
					vars,
					h.variableName,
					authPath...,
				)
				if authErr != nil && !variables.IsNotFoundError(authErr) {
					return authErr
				}
				authCfg = loadedAuth
			}

			desiredArgs, err := buildAndValidateExtraArgs(sai, authCfg)
			if err != nil {
				return err
			}

			apiServer := &obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer
			if err := validateNoConflictingExtraArgs(apiServer.ExtraArgs, desiredArgs); err != nil {
				return err
			}

			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("adding ServiceAccount issuer configuration")

			for _, desired := range desiredArgs {
				apiServer.ExtraArgs = ensureArg(apiServer.ExtraArgs, desired.Name, *desired.Value)
			}

			return nil
		},
	)
}

func buildAndValidateExtraArgs(
	sai *v1alpha1.ServiceAccountIssuer,
	authCfg *v1alpha1.Authentication,
) ([]bootstrapv1.Arg, error) {
	fldPath := field.NewPath(VariableName)
	var allErrs field.ErrorList

	allErrs = append(allErrs, webhook.ValidateWebhookURL(fldPath.Child("issuer"), sai.Issuer, true)...)
	allErrs = append(allErrs, validateURLHostAndPort(fldPath.Child("issuer"), sai.Issuer)...)

	if authCfg != nil {
		for i := range authCfg.JWT {
			if authCfg.JWT[i].Issuer.URL == sai.Issuer {
				allErrs = append(
					allErrs,
					field.Invalid(
						fldPath.Child("issuer"),
						sai.Issuer,
						fmt.Sprintf(
							"URL must not overlap with authentication.jwt[%d].issuer.url",
							i,
						),
					),
				)
			}
		}
	}

	desired := make([]bootstrapv1.Arg, 0, 4)
	issuerVal := sai.Issuer
	desired = append(desired, bootstrapv1.Arg{
		Name:  ServiceAccountIssuerArg,
		Value: &issuerVal,
	})

	var sortedAudiences []string
	if len(sai.Audiences) > 0 {
		sortedAudiences = slices.Clone(sai.Audiences)
		slices.Sort(sortedAudiences)
		for i, aud := range sortedAudiences {
			if aud == "" {
				allErrs = append(allErrs, field.Required(fldPath.Child("audiences").Index(i), ""))
			} else if strings.IndexFunc(aud, unicode.IsSpace) >= 0 {
				allErrs = append(
					allErrs,
					field.Invalid(
						fldPath.Child("audiences").Index(i),
						aud,
						"audience must not contain whitespace",
					),
				)
			}
		}
		audiencesVal := strings.Join(sortedAudiences, ",")
		desired = append(desired, bootstrapv1.Arg{
			Name:  APIAudiencesArg,
			Value: &audiencesVal,
		})
	}

	if sai.JWKSURI != "" {
		allErrs = append(allErrs, validateJWKSURI(fldPath.Child("jwksURI"), sai.JWKSURI)...)
		jwksVal := sai.JWKSURI
		desired = append(desired, bootstrapv1.Arg{
			Name:  ServiceAccountJWKSURIArg,
			Value: &jwksVal,
		})
	}

	if sai.MaxTokenExpiration != nil {
		d := sai.MaxTokenExpiration.Duration
		if d < minServiceAccountTokenExpiration || d > maxServiceAccountTokenExpiration {
			allErrs = append(
				allErrs,
				field.Invalid(
					fldPath.Child("maxTokenExpiration"),
					d.String(),
					fmt.Sprintf(
						"must be between %s and %s",
						minServiceAccountTokenExpiration,
						maxServiceAccountTokenExpiration,
					),
				),
			)
		}
		expVal := d.String()
		desired = append(desired, bootstrapv1.Arg{
			Name:  ServiceAccountMaxTokenExpArg,
			Value: &expVal,
		})
	}

	if len(allErrs) > 0 {
		return nil, allErrs.ToAggregate()
	}

	// Verify that kube-apiserver's exact CLI parser (spf13/pflag) parses the rendered flags
	// and round-trips CSV --api-audiences without quote errors or comma-splitting drift.
	if err := validateRenderedKubeAPIServerFlags(desired, sortedAudiences); err != nil {
		return nil, err
	}

	return desired, nil
}

func validateJWKSURI(fldPath *field.Path, rawURL string) field.ErrorList {
	u, err := url.Parse(rawURL)
	if err != nil {
		return webhook.ValidateWebhookURL(fldPath, rawURL, true)
	}
	// OpenID Connect Discovery 1.0 Section 3 and kube-apiserver permit query parameters on
	// jwks_uri while still requiring https, a host, no userinfo, and no fragment.
	withoutQuery := *u
	withoutQuery.RawQuery = ""
	withoutQuery.ForceQuery = false

	return append(
		webhook.ValidateWebhookURL(fldPath, withoutQuery.String(), true),
		validateURLHostAndPort(fldPath, rawURL)...,
	)
}

func validateURLHostAndPort(fldPath *field.Path, rawURL string) field.ErrorList {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	var allErrs field.ErrorList
	if u.Hostname() == "" {
		allErrs = append(allErrs, field.Invalid(fldPath, rawURL, "hostname must not be empty"))
	}
	if portStr := u.Port(); portStr != "" {
		portNum, convErr := strconv.Atoi(portStr)
		if convErr != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, portStr, "port must be a valid integer"))
		} else if portErrs := utilvalidation.IsValidPortNum(portNum); len(portErrs) > 0 {
			allErrs = append(
				allErrs,
				field.Invalid(fldPath, portNum, "port is not valid: "+strings.Join(portErrs, ", ")),
			)
		}
	}
	return allErrs
}

func validateRenderedKubeAPIServerFlags(
	desired []bootstrapv1.Arg,
	expectedAudiences []string,
) error {
	fs := pflag.NewFlagSet("kube-apiserver", pflag.ContinueOnError)
	var (
		parsedIssuers   []string
		parsedAudiences []string
		parsedJWKSURI   string
		parsedMaxExp    time.Duration
	)
	fs.StringArrayVar(&parsedIssuers, ServiceAccountIssuerArg, nil, "")
	fs.StringSliceVar(&parsedAudiences, APIAudiencesArg, nil, "")
	fs.StringVar(&parsedJWKSURI, ServiceAccountJWKSURIArg, "", "")
	fs.DurationVar(&parsedMaxExp, ServiceAccountMaxTokenExpArg, 0, "")

	rawFlags := make([]string, 0, len(desired))
	for _, arg := range desired {
		rawFlags = append(rawFlags, fmt.Sprintf("--%s=%s", arg.Name, *arg.Value))
	}
	if err := fs.Parse(rawFlags); err != nil {
		return fmt.Errorf("rendered ServiceAccount issuer flags failed pflag validation: %w", err)
	}
	if len(expectedAudiences) > 0 && !slices.Equal(parsedAudiences, expectedAudiences) {
		return fmt.Errorf(
			"rendered --%s value did not round-trip cleanly (got %v, expected %v); "+
				"entries must not contain commas or CSV quotes",
			APIAudiencesArg,
			parsedAudiences,
			expectedAudiences,
		)
	}
	return nil
}

func isManagedExtraArg(name string) bool {
	switch name {
	case ServiceAccountIssuerArg,
		APIAudiencesArg,
		ServiceAccountJWKSURIArg,
		ServiceAccountMaxTokenExpArg:
		return true
	default:
		return false
	}
}

func validateNoConflictingExtraArgs(
	existing []bootstrapv1.Arg,
	desired []bootstrapv1.Arg,
) error {
	desiredByName := make(map[string]string, len(desired))
	for _, arg := range desired {
		desiredByName[arg.Name] = *arg.Value
	}

	seen := make(map[string]int, len(desired))
	for _, arg := range existing {
		if !isManagedExtraArg(arg.Name) {
			continue
		}
		seen[arg.Name]++
		if seen[arg.Name] > 1 {
			return fmt.Errorf(
				"found duplicate existing extraArgs %q which conflicts with serviceAccountIssuer; "+
					"remove the direct extraArgs and use clusterConfig.serviceAccountIssuer instead",
				arg.Name,
			)
		}
		wantVal, ok := desiredByName[arg.Name]
		if !ok {
			return fmt.Errorf(
				"found existing extraArgs %q which conflicts with serviceAccountIssuer; "+
					"remove the direct extraArgs and use clusterConfig.serviceAccountIssuer instead",
				arg.Name,
			)
		}
		if arg.Value == nil {
			return fmt.Errorf(
				"existing --%s extraArg has nil value; expected %q",
				arg.Name,
				wantVal,
			)
		}
		if *arg.Value != wantVal {
			return fmt.Errorf(
				"found existing extraArgs %q with value %q which conflicts with "+
					"serviceAccountIssuer value %q; remove the direct extraArgs and use "+
					"clusterConfig.serviceAccountIssuer instead",
				arg.Name,
				*arg.Value,
				wantVal,
			)
		}
	}

	return nil
}

func ensureArg(args []bootstrapv1.Arg, name, value string) []bootstrapv1.Arg {
	for i := range args {
		if args[i].Name == name {
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
