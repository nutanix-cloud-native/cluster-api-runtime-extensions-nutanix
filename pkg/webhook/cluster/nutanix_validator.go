// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"fmt"
	"net"
	"net/http"

	v1 "k8s.io/api/admission/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/helpers"
)

type nutanixValidator struct {
	client  ctrlclient.Client
	decoder admission.Decoder
}

func NewNutanixValidator(
	client ctrlclient.Client, decoder admission.Decoder,
) *nutanixValidator {
	return &nutanixValidator{
		client:  client,
		decoder: decoder,
	}
}

func (a *nutanixValidator) Validator() admission.HandlerFunc {
	return a.validate
}

func (a *nutanixValidator) validate(
	ctx context.Context,
	req admission.Request,
) admission.Response {
	if req.Operation == v1.Delete {
		return admission.Allowed("")
	}

	cluster := &clusterv1.Cluster{}
	err := a.decoder.Decode(req, cluster)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if cluster.Spec.Topology == nil {
		return admission.Allowed("")
	}

	if utils.GetProvider(cluster) != "nutanix" {
		return admission.Allowed("")
	}

	clusterConfig, err := variables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return admission.Denied(
			fmt.Errorf("failed to unmarshal cluster topology variable %q: %w",
				v1alpha1.ClusterConfigVariableName,
				err).Error(),
		)
	}

	if clusterConfig.Nutanix != nil &&
		clusterConfig.Addons != nil {
		// Check if Prism Central IP is in MetalLB Load Balancer IP range.
		if err := checkIfPrismCentralIPInLoadBalancerIPRange(
			clusterConfig.Nutanix.PrismCentralEndpoint,
			clusterConfig.Addons.ServiceLoadBalancer,
		); err != nil {
			return admission.Denied(err.Error())
		}
	}

	return admission.Allowed("")
}

// checkIfPrismCentralIPInLoadBalancerIPRange checks if the Prism Central IP is in the MetalLB Load Balancer IP range.
// Errors out if Prism Central IP is in the Load Balancer IP range.
func checkIfPrismCentralIPInLoadBalancerIPRange(
	pcEndpoint v1alpha1.NutanixPrismCentralEndpointSpec,
	serviceLoadBalancerConfiguration *v1alpha1.ServiceLoadBalancer,
) error {
	if serviceLoadBalancerConfiguration == nil ||
		serviceLoadBalancerConfiguration.Provider != v1alpha1.ServiceLoadBalancerProviderMetalLB ||
		serviceLoadBalancerConfiguration.Configuration == nil {
		return nil
	}

	pcHostname, _, err := pcEndpoint.ParseURL()
	if err != nil {
		return err
	}

	pcIP := net.ParseIP(pcHostname)
	// PC URL can contain IP/FQDN, so compare only if PC is an IP address.
	if pcIP == nil {
		return nil
	}

	for _, pool := range serviceLoadBalancerConfiguration.Configuration.AddressRanges {
		isIPInRange, err := helpers.IsIPInRange(pool.Start, pool.End, pcIP.String())
		if err != nil {
			return fmt.Errorf(
				"error while checking if Prism Central IP %q is part of MetalLB address range %q-%q: %w",
				pcIP,
				pool.Start,
				pool.End,
				err,
			)
		}
		if isIPInRange {
			return fmt.Errorf("prism central IP %q must not be part of MetalLB address range %q-%q",
				pcIP, pool.Start, pool.End)
		}
	}

	return nil
}
