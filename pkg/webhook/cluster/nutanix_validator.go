// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"

	v1 "k8s.io/api/admission/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
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

	if clusterConfig.Nutanix != nil {
		// Check if Prism Central and Control Plane IP are same.
		if err := checkIfPrismCentralAndControlPlaneIPSame(
			clusterConfig.Nutanix.PrismCentralEndpoint.URL,
			clusterConfig.Nutanix.ControlPlaneEndpoint.Host,
		); err != nil {
			return admission.Denied(err.Error())
		}
	}

	return admission.Allowed("")
}

// checkIfPrismCentralAndControlPlaneIPSame checks if Prism Central and Control Plane IP are same.
// It compares strictly IP addresses(no FQDN) and doesn't involve any network calls.
// This is a temporary check until we have a better way to handle this by reserving IPs
// using IPAM provider.
func checkIfPrismCentralAndControlPlaneIPSame(
	pcRawURL string,
	controlPlaneEndpointHost string,
) error {
	controlPlaneEndpointIP := net.ParseIP(controlPlaneEndpointHost)
	if controlPlaneEndpointIP == nil {
		// controlPlaneEndpointIP is strictly accepted as an IP address from user so
		// if it is not an IP address, it is invalid.
		return fmt.Errorf("invalid Nutanix control plane endpoint IP %q",
			controlPlaneEndpointHost)
	}

	pcURL, err := url.ParseRequestURI(pcRawURL)
	if err != nil {
		return fmt.Errorf("failed to parse Prism Central URL %q: %w",
			pcURL,
			err)
	}

	pcHost, _, err := net.SplitHostPort(pcURL.Host)
	if err != nil {
		return fmt.Errorf("failed to parse Prism Central host %q: %w",
			pcURL.Host,
			err)
	}

	pcIP := net.ParseIP(pcHost)
	// PC URL can contain IP/FQDN, so compare only if PC is an IP address.
	if pcIP != nil && pcIP.Equal(controlPlaneEndpointIP) {
		return fmt.Errorf("prism central and control plane endpoint cannot have the same IP %q",
			pcIP)
	}

	return nil
}
