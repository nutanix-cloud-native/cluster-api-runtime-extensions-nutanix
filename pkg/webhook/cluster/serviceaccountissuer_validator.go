// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

	v1 "k8s.io/api/admission/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/kubeadm/authentication"
)

type serviceAccountIssuerValidator struct {
	decoder admission.Decoder
}

func NewServiceAccountIssuerValidator(decoder admission.Decoder) *serviceAccountIssuerValidator {
	return &serviceAccountIssuerValidator{
		decoder: decoder,
	}
}

func (s *serviceAccountIssuerValidator) Validator() admission.HandlerFunc {
	return s.validate
}

func (s *serviceAccountIssuerValidator) validate(
	_ context.Context,
	req admission.Request,
) admission.Response {
	if req.Operation == v1.Delete {
		return admission.Allowed("")
	}

	cluster := &clusterv1.Cluster{}
	if err := s.decoder.Decode(req, cluster); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if !cluster.Spec.Topology.IsDefined() {
		return admission.Allowed("")
	}

	clusterConfig, err := variables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return admission.Denied(
			fmt.Errorf(
				"failed to unmarshal cluster topology variable %q: %w",
				v1alpha1.ClusterConfigVariableName,
				err,
			).Error(),
		)
	}

	if clusterConfig == nil || clusterConfig.ServiceAccountIssuer == nil {
		return admission.Allowed("")
	}

	sai := clusterConfig.ServiceAccountIssuer
	defaultK8sAudience := authentication.DefaultServiceAccountIssuers(
		cluster.Spec.ClusterNetwork.ServiceDomain,
	)[0]

	var warnings []string
	missingInClusterAudience := false
	if len(sai.Audiences) > 0 &&
		!slices.Contains(sai.Audiences, sai.Issuer) &&
		!slices.Contains(sai.Audiences, defaultK8sAudience) {
		missingInClusterAudience = true
		warnings = append(
			warnings,
			fmt.Sprintf(
				"clusterConfig.serviceAccountIssuer.audiences does not include "+
					"serviceAccountIssuer.issuer (%q) or the default Kubernetes audience (%q); "+
					"setting only external audiences rejects in-cluster ServiceAccount tokens "+
					"bound to the default audience until refreshed",
				sai.Issuer,
				defaultK8sAudience,
			),
		)
	}

	if req.Operation == v1.Update && len(req.OldObject.Raw) > 0 {
		oldCluster := &clusterv1.Cluster{}
		if err := s.decoder.DecodeRaw(req.OldObject, oldCluster); err != nil {
			return admission.Errored(
				http.StatusBadRequest,
				fmt.Errorf("failed to decode old cluster: %w", err),
			)
		}

		if oldCluster.Spec.Topology.IsDefined() {
			oldConfig, oldErr := variables.UnmarshalClusterConfigVariable(
				oldCluster.Spec.Topology.Variables,
			)
			if oldErr == nil {
				var oldSAI *v1alpha1.ServiceAccountIssuer
				if oldConfig != nil {
					oldSAI = oldConfig.ServiceAccountIssuer
				}
				if len(sai.Audiences) > 0 || (oldSAI != nil && len(oldSAI.Audiences) > 0) {
					oldAudiences := effectiveServiceAccountAudiences(
						oldSAI,
						oldCluster.Spec.ClusterNetwork.ServiceDomain,
					)
					newAudiences := effectiveServiceAccountAudiences(
						sai,
						cluster.Spec.ClusterNetwork.ServiceDomain,
					)

					var removed []string
					for _, aud := range oldAudiences {
						if slices.Contains(newAudiences, aud) || slices.Contains(removed, aud) {
							continue
						}
						if missingInClusterAudience &&
							(aud == sai.Issuer || aud == defaultK8sAudience) {
							continue
						}
						removed = append(removed, aud)
					}
					slices.Sort(removed)
					if len(removed) > 0 {
						warnings = append(
							warnings,
							fmt.Sprintf(
								"clusterConfig.serviceAccountIssuer.audiences removes previously "+
									"accepted audience(s) %q; existing ServiceAccount tokens bound "+
									"only to removed audiences will be rejected until refreshed",
								strings.Join(removed, ", "),
							),
						)
					}
				}
			}
		}
	}

	if len(warnings) > 0 {
		return admission.Allowed("").WithWarnings(warnings...)
	}
	return admission.Allowed("")
}

func effectiveServiceAccountAudiences(
	sai *v1alpha1.ServiceAccountIssuer,
	serviceDomain string,
) []string {
	if sai != nil {
		if len(sai.Audiences) > 0 {
			return sai.Audiences
		}
		if sai.Issuer != "" {
			return []string{sai.Issuer}
		}
	}
	return authentication.DefaultServiceAccountIssuers(serviceDomain)
}
