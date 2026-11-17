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
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
)

// ipamProviderValidator refuses automatic address reservation on infrastructure
// that cannot honour it.
//
// Reserving addresses is implemented by two Nutanix components: the Nutanix CCM
// reserves Service addresses with its AutoIPAMMetalLB strategy, and the Nutanix
// IPAM provider reserves the API server address. Neither has an equivalent
// anywhere else, and the AWS, EKS, Docker and host-agent CCMs ignore the
// setting entirely.
//
// The fields are still offered on every provider's cluster config because
// serviceLoadBalancer is one type shared across all of them. Left unchecked, a
// cluster asking for a reserved address on AWS passes validation, reserves
// nothing, and its Services never get an address - a silent hang with nothing
// pointing at the cause. Refusing it up front, when the cluster is created,
// turns that into one sentence the operator reads immediately.
type ipamProviderValidator struct {
	client  ctrlclient.Client
	decoder admission.Decoder
}

func NewIPAMProviderValidator(
	client ctrlclient.Client, decoder admission.Decoder,
) *ipamProviderValidator {
	return &ipamProviderValidator{client: client, decoder: decoder}
}

func (a *ipamProviderValidator) Validator() admission.HandlerFunc {
	return a.validate
}

func (a *ipamProviderValidator) validate(
	_ context.Context,
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
	if utils.GetProvider(cluster) == "nutanix" {
		return admission.Allowed("")
	}

	clusterConfig, err := variables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return admission.Denied(
			fmt.Errorf("failed to unmarshal cluster topology variable %q: %w",
				v1alpha1.ClusterConfigVariableName, err).Error(),
		)
	}

	if msg := unsupportedReservationOnProvider(utils.GetProvider(cluster), clusterConfig); msg != "" {
		return admission.Denied(msg)
	}
	return admission.Allowed("")
}

// unsupportedReservationOnProvider returns the reason a cluster may not reserve
// addresses on provider, or an empty string when it may.
//
// Kept separate from the admission plumbing so the rule itself is testable
// without building a decoder, and so the message is one string rather than
// something assembled across branches.
func unsupportedReservationOnProvider(
	provider string,
	clusterConfig *variables.ClusterConfigSpec,
) string {
	if provider == "nutanix" || clusterConfig == nil {
		return ""
	}

	if addons := clusterConfig.Addons; addons != nil {
		if slb := addons.ServiceLoadBalancer; slb != nil &&
			slb.Configuration != nil &&
			slb.Configuration.AutoIPAM != nil {
			return fmt.Sprintf(
				"spec.topology.variables[%s].addons.serviceLoadBalancer.configuration.autoIPAM "+
					"is only supported on Nutanix clusters, and this cluster is %q. "+
					"Reserving Service addresses is implemented by the Nutanix CCM, which this "+
					"cluster does not run; use addressRanges instead.",
				v1alpha1.ClusterConfigVariableName, provider,
			)
		}
	}

	// Belt and braces. A non-Nutanix cluster config carries no nutanix section
	// today, but the control plane endpoint type is shared too, so the same
	// silent failure would be one refactor away.
	if clusterConfig.Nutanix != nil {
		if vip := clusterConfig.Nutanix.ControlPlaneEndpoint.VirtualIPSpec; vip != nil &&
			vip.Configuration != nil &&
			vip.Configuration.IPReservation != nil {
			return fmt.Sprintf(
				"spec.topology.variables[%s].nutanix.controlPlaneEndpoint.virtualIP.configuration."+
					"ipReservation is only supported on Nutanix clusters, and this cluster is %q. "+
					"Reserving the API server address needs the Nutanix IPAM provider; "+
					"set a host instead.",
				v1alpha1.ClusterConfigVariableName, provider,
			)
		}
	}

	return ""
}
