// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package serviceaccountissuer

import (
	"context"
	"fmt"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	ServiceAccountIssuerArg  = "service-account-issuer"
	APIAudiencesArg          = "api-audiences"
	ServiceAccountJWKSURIArg = "service-account-jwks-uri"
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
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("adding ServiceAccount issuer configuration")

			apiServer := &obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer

			// Conflict detection: fail if user already set any of our flags directly via extraArgs.
			for _, arg := range apiServer.ExtraArgs {
				if arg.Name == ServiceAccountIssuerArg ||
					arg.Name == APIAudiencesArg ||
					arg.Name == ServiceAccountJWKSURIArg {
					return fmt.Errorf(
						"found existing extraArgs %q which conflicts with serviceAccountIssuer; "+
							"remove the direct extraArgs and use clusterConfig.serviceAccountIssuer instead",
						arg.Name,
					)
				}
			}

			issuerVal := sai.Issuer
			apiServer.ExtraArgs = append(apiServer.ExtraArgs, bootstrapv1.Arg{
				Name:  ServiceAccountIssuerArg,
				Value: &issuerVal,
			})

			if len(sai.Audiences) > 0 {
				audiencesVal := strings.Join(sai.Audiences, ",")
				apiServer.ExtraArgs = append(apiServer.ExtraArgs, bootstrapv1.Arg{
					Name:  APIAudiencesArg,
					Value: &audiencesVal,
				})
			}

			if sai.JWKSURI != "" {
				jwksVal := sai.JWKSURI
				apiServer.ExtraArgs = append(apiServer.ExtraArgs, bootstrapv1.Arg{
					Name:  ServiceAccountJWKSURIArg,
					Value: &jwksVal,
				})
			}

			return nil
		},
	)
}
