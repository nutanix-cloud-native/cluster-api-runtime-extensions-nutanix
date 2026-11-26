// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"cmp"
	"context"
	"fmt"
	"net/http"

	v1 "k8s.io/api/admission/v1"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/kubeadm/authentication"
)

const kubeadmControlPlaneTemplateKind = "KubeadmControlPlaneTemplate"

type authenticationValidator struct {
	client  ctrlclient.Client
	decoder admission.Decoder
}

func NewAuthenticationValidator(
	client ctrlclient.Client, decoder admission.Decoder,
) *authenticationValidator {
	return &authenticationValidator{
		client:  client,
		decoder: decoder,
	}
}

func (a *authenticationValidator) Validator() admission.HandlerFunc {
	return a.validate
}

func (a *authenticationValidator) validate(
	ctx context.Context,
	req admission.Request,
) admission.Response {
	if req.Operation == v1.Delete {
		return admission.Allowed("")
	}

	cluster := &clusterv1.Cluster{}
	if err := a.decoder.Decode(req, cluster); err != nil {
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

	if clusterConfig == nil ||
		clusterConfig.Authentication == nil ||
		len(clusterConfig.Authentication.JWT) == 0 {
		return admission.Allowed("")
	}

	if err := authentication.ValidateKubernetesVersion(cluster.Spec.Topology.Version); err != nil {
		return admission.Denied(fmt.Sprintf("clusterConfig.authentication: %v", err))
	}

	disallowedIssuers, err := a.resolveDisallowedIssuersAndValidateTemplate(ctx, cluster)
	if err != nil {
		return admission.Denied(fmt.Sprintf("clusterConfig.authentication: %v", err))
	}

	if err := authentication.ValidateConfiguration(
		clusterConfig.Authentication,
		disallowedIssuers,
	); err != nil {
		return admission.Denied(fmt.Sprintf("clusterConfig.authentication: %v", err))
	}

	return admission.Allowed("")
}

func (a *authenticationValidator) resolveDisallowedIssuersAndValidateTemplate(
	ctx context.Context,
	cluster *clusterv1.Cluster,
) ([]string, error) {
	serviceDomain := cluster.Spec.ClusterNetwork.ServiceDomain
	defaultIssuers := authentication.DefaultServiceAccountIssuers(serviceDomain)

	if a.client == nil || cluster.Spec.Topology.ClassRef.Name == "" {
		return defaultIssuers, nil
	}

	classKey := ctrlclient.ObjectKey{
		Namespace: cmp.Or(cluster.Spec.Topology.ClassRef.Namespace, cluster.Namespace),
		Name:      cluster.Spec.Topology.ClassRef.Name,
	}
	cc := &clusterv1.ClusterClass{}
	if err := a.client.Get(ctx, classKey, cc); err != nil {
		if ctrlclient.IgnoreNotFound(err) == nil {
			return defaultIssuers, nil
		}
		return nil, fmt.Errorf("failed to get ClusterClass %s: %w", classKey, err)
	}

	if cc.Spec.ControlPlane.TemplateRef.Kind != kubeadmControlPlaneTemplateKind ||
		cc.Spec.ControlPlane.TemplateRef.Name == "" {
		return defaultIssuers, nil
	}

	templateKey := ctrlclient.ObjectKey{
		Namespace: cc.Namespace,
		Name:      cc.Spec.ControlPlane.TemplateRef.Name,
	}
	cpTemplate := &controlplanev1.KubeadmControlPlaneTemplate{}
	if err := a.client.Get(ctx, templateKey, cpTemplate); err != nil {
		if ctrlclient.IgnoreNotFound(err) == nil {
			return defaultIssuers, nil
		}
		return nil, fmt.Errorf("failed to get KubeadmControlPlaneTemplate %s: %w", templateKey, err)
	}

	return authentication.ValidateControlPlaneTemplate(cpTemplate, serviceDomain)
}
