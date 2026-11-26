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
	clusterConfig, err := variables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return admission.Denied(
			fmt.Errorf("failed to unmarshal cluster topology variable %q: %w",
				v1alpha1.ClusterConfigVariableName, err).Error(),
		)
	}

	if msg := unsupportedReservationOnProvider(clusterProvider(cluster, clusterConfig), clusterConfig); msg != "" {
		return admission.Denied(msg)
	}
	if msg := conflictingEndpointSources(clusterConfig); msg != "" {
		return admission.Denied(msg)
	}
	return admission.Allowed("")
}

// conflictingEndpointSources returns the reason a cluster may not both name its
// API server address and reserve one, or an empty string when it does only one.
//
// kube-vip binds the virtual IP configuration's address, falling back to the
// endpoint host. Accepting both would bind the host - an FQDN it cannot bind at
// all, or an external floating address that is mapped to the node network
// rather than present on it - and quietly ignore the reservation. The two are
// alternatives, so say so at create instead.
func conflictingEndpointSources(clusterConfig *variables.ClusterConfigSpec) string {
	if clusterConfig == nil || clusterConfig.Nutanix == nil {
		return ""
	}
	endpoint := clusterConfig.Nutanix.ControlPlaneEndpoint
	if endpoint.Host == "" {
		return ""
	}
	vip := endpoint.VirtualIPSpec
	if vip == nil || vip.Configuration == nil || vip.Configuration.IPReservation == nil {
		return ""
	}
	return fmt.Sprintf(
		"spec.topology.variables[%s].nutanix.controlPlaneEndpoint sets both host %q and "+
			"virtualIP.configuration.ipReservation. Name the API server address or reserve "+
			"one, not both: the reserved address would be ignored and kube-vip would be "+
			"configured with the host, which it cannot bind when the host is an FQDN or an "+
			"address outside the control plane machines' network.",
		v1alpha1.ClusterConfigVariableName, endpoint.Host,
	)
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
	// Only refuse when the cluster is positively known to be something other
	// than Nutanix. An unrecognised provider is left alone deliberately: this
	// runs on create, and refusing what it cannot classify would block valid
	// clusters, which is worse than missing a misconfiguration that the
	// reservation itself will then fail on.
	if provider == providerNutanix || provider == "" || clusterConfig == nil {
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

const providerNutanix = "nutanix"

// clusterProvider names the infrastructure a cluster runs on, or an empty
// string when that cannot be told yet.
//
// The provider label is the obvious source but it is applied after admission,
// so on a newly created cluster it is not there yet - which is exactly when
// this check runs, and reading only the label refused a valid Nutanix cluster
// for being provider "". The cluster config carries a section per provider and
// is present from the start, so it is the reliable signal here and the label is
// only a fallback for a cluster whose config names no provider at all.
func clusterProvider(
	cluster *clusterv1.Cluster,
	clusterConfig *variables.ClusterConfigSpec,
) string {
	if clusterConfig != nil {
		switch {
		case clusterConfig.Nutanix != nil:
			return providerNutanix
		case clusterConfig.AWS != nil:
			return "aws"
		case clusterConfig.EKS != nil:
			return "eks"
		case clusterConfig.Docker != nil:
			return "docker"
		}
	}
	return utils.GetProvider(cluster)
}
