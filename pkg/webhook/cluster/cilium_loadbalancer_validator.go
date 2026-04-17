// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"fmt"
	"net/http"

	v1 "k8s.io/api/admission/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

type ciliumLoadBalancerValidator struct {
	client  ctrlclient.Client
	decoder admission.Decoder
}

func NewCiliumLoadBalancerValidator(
	client ctrlclient.Client,
	decoder admission.Decoder,
) *ciliumLoadBalancerValidator {
	return &ciliumLoadBalancerValidator{client: client, decoder: decoder}
}

func (v *ciliumLoadBalancerValidator) Validator() admission.HandlerFunc {
	return v.validate
}

func (v *ciliumLoadBalancerValidator) validate(
	_ context.Context,
	req admission.Request,
) admission.Response {
	if req.Operation == v1.Delete {
		return admission.Allowed("")
	}

	cluster := &clusterv1.Cluster{}
	if err := v.decoder.Decode(req, cluster); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	if !cluster.Spec.Topology.IsDefined() {
		return admission.Allowed("")
	}

	if cluster.Annotations[v1alpha1.PreflightChecksSkipAnnotationKey] ==
		v1alpha1.PreflightChecksSkipAllAnnotationValue {
		return admission.Allowed("")
	}

	cfg, err := variables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return admission.Denied(fmt.Errorf(
			"failed to unmarshal cluster topology variable %q: %w",
			v1alpha1.ClusterConfigVariableName, err,
		).Error())
	}
	if cfg == nil || cfg.Addons == nil || cfg.Addons.ServiceLoadBalancer == nil {
		return admission.Allowed("")
	}
	if cfg.Addons.ServiceLoadBalancer.Provider != v1alpha1.ServiceLoadBalancerProviderCilium {
		return admission.Allowed("")
	}

	if cfg.Addons.CNI == nil || cfg.Addons.CNI.Provider != v1alpha1.CNIProviderCilium {
		return admission.Denied(fmt.Sprintf(
			"ServiceLoadBalancer provider %q requires Cilium CNI (addons.cni.provider=%s)",
			v1alpha1.ServiceLoadBalancerProviderCilium,
			v1alpha1.CNIProviderCilium,
		))
	}
	if cfg.KubeProxy == nil || cfg.KubeProxy.Mode != v1alpha1.KubeProxyModeDisabled {
		return admission.Denied(fmt.Sprintf(
			"ServiceLoadBalancer provider %q requires kube-proxy to be disabled (kubeProxy.mode=%s)",
			v1alpha1.ServiceLoadBalancerProviderCilium,
			v1alpha1.KubeProxyModeDisabled,
		))
	}
	return admission.Allowed("")
}
