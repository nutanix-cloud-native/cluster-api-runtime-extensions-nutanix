// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"

	v1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
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

	if clusterConfig.Nutanix != nil {
		if err := validatePrismCentralIPDoesNotEqualControlPlaneIP(
			clusterConfig.Nutanix.PrismCentralEndpoint,
			clusterConfig.Nutanix.ControlPlaneEndpoint,
		); err != nil {
			return admission.Denied(err.Error())
		}

		if clusterConfig.Addons != nil {
			// Check if Prism Central IP is in MetalLB Load Balancer IP range.
			if err := validatePrismCentralIPNotInLoadBalancerIPRange(
				clusterConfig.Nutanix.PrismCentralEndpoint,
				clusterConfig.Addons.ServiceLoadBalancer,
			); err != nil {
				return admission.Denied(err.Error())
			}
		}
	}

	if err := validateFailureDomainRelatedConfig(clusterConfig, cluster); err != nil {
		return admission.Denied(err.Error())
	}

	return admission.Allowed("")
}

// validateFailureDomainRelatedConfig validates the failure domain related configuration in cluster topology.
func validateFailureDomainRelatedConfig(
	clusterConfig *variables.ClusterConfigSpec,
	cluster *clusterv1.Cluster,
) error {
	fldErrs := field.ErrorList{}
	fldPath := field.NewPath("spec", "topology")

	// Validate that either failureDomains is set, or cluster and subnets are set with machineDetails, for control plane.
	if clusterConfig.ControlPlane != nil &&
		clusterConfig.ControlPlane.Nutanix != nil &&
		len(clusterConfig.ControlPlane.Nutanix.FailureDomains) == 0 {

		machineDetails := clusterConfig.ControlPlane.Nutanix.MachineDetails
		if machineDetails.Cluster == nil || !(machineDetails.Cluster.IsName() || machineDetails.Cluster.IsUUID()) {
			fldErrs = append(fldErrs, field.Required(
				fldPath.Child(
					"variables",
					"clusterConfig",
					"value",
					"controlPlane",
					"nutanix",
					"machineDetails",
					"cluster",
				),
				"\"cluster\" must set when failureDomains is not configured.",
			))
		}

		if len(machineDetails.Subnets) == 0 {
			fldErrs = append(fldErrs, field.Required(
				fldPath.Child(
					"variables",
					"clusterConfig",
					"value",
					"controlPlane",
					"nutanix",
					"machineDetails",
					"subnets",
				),
				"\"subnets\" must set when failureDomains is not configured.",
			))
		}
	}

	// Validate either failureDomains is set, or cluster and sebnets are set with machineDetails, for workers.
	if cluster.Spec.Topology.Workers != nil {
		for i := range cluster.Spec.Topology.Workers.MachineDeployments {
			md := cluster.Spec.Topology.Workers.MachineDeployments[i]
			if md.FailureDomain != nil && *md.FailureDomain != "" {
				// failureDomain is configured
				continue
			}

			if md.Variables != nil && len(md.Variables.Overrides) > 0 {
				workerConfig, err := variables.UnmarshalWorkerConfigVariable(md.Variables.Overrides)
				if err != nil {
					fldErrs = append(fldErrs, field.InternalError(
						fldPath.Child("workers", "machineDeployments", "variables", "workerConfig"),
						fmt.Errorf("failed to unmarshal worker topology variable: %w", err)))
				}
				if workerConfig.Nutanix == nil {
					continue
				}

				machineDetails := workerConfig.Nutanix.MachineDetails
				if machineDetails.Cluster == nil ||
					!(machineDetails.Cluster.IsName() || machineDetails.Cluster.IsUUID()) {
					fldErrs = append(fldErrs, field.Required(
						fldPath.Child(
							"workers",
							"machineDeployments",
							"variables",
							"workerConfig",
							"nutanix",
							"machineDetails",
							"cluster",
						),
						"\"cluster\" must set when failureDomain is not configured.",
					))
				}
				if len(machineDetails.Subnets) == 0 {
					fldErrs = append(fldErrs, field.Required(
						fldPath.Child(
							"workers",
							"machineDeployments",
							"variables",
							"workerConfig",
							"nutanix",
							"machineDetails",
							"subnets",
						),
						"\"subnets\" must set when failureDomain is not configured.",
					))
				}
			}
		}
	}

	return fldErrs.ToAggregate()
}

// validatePrismCentralIPNotInLoadBalancerIPRange checks if the Prism Central IP is not
// in the MetalLB Load Balancer IP range and error out if it is.
func validatePrismCentralIPNotInLoadBalancerIPRange(
	pcEndpoint v1alpha1.NutanixPrismCentralEndpointSpec,
	serviceLoadBalancerConfiguration *v1alpha1.ServiceLoadBalancer,
) error {
	if serviceLoadBalancerConfiguration == nil ||
		serviceLoadBalancerConfiguration.Provider != v1alpha1.ServiceLoadBalancerProviderMetalLB ||
		serviceLoadBalancerConfiguration.Configuration == nil {
		return nil
	}

	pcIP, err := pcEndpoint.ParseIP()
	if err != nil {
		// If it's not able to parse IP correctly then, ignore the error as
		// we want to compare only IP addresses.
		return nil
	}

	for _, pool := range serviceLoadBalancerConfiguration.Configuration.AddressRanges {
		isIPInRange, err := helpers.IsIPInRange(pool.Start, pool.End, pcIP.String())
		if err != nil {
			return fmt.Errorf(
				"failed to check if Prism Central IP %q is part of MetalLB address range %q-%q: %w",
				pcIP,
				pool.Start,
				pool.End,
				err,
			)
		}
		if isIPInRange {
			errMsg := fmt.Sprintf(
				"Prism Central IP %q must not be part of MetalLB address range %q-%q",
				pcIP,
				pool.Start,
				pool.End,
			)
			return errors.New(errMsg)
		}
	}

	return nil
}

// validatePrismCentralIPDoesNotEqualControlPlaneIP checks if Prism Central and Control Plane IP are same,
// error out if they are same.
// It strictly compares IP addresses(no FQDN) and doesn't involve any network calls.
func validatePrismCentralIPDoesNotEqualControlPlaneIP(
	pcEndpoint v1alpha1.NutanixPrismCentralEndpointSpec,
	controlPlaneEndpointSpec v1alpha1.ControlPlaneEndpointSpec,
) error {
	controlPlaneVIP, err := netip.ParseAddr(controlPlaneEndpointSpec.VirtualIPAddress())
	if err != nil {
		// If controlPlaneEndpointIP is a hostname, we cannot compare it with PC IP
		// so return directly.
		return nil
	}

	pcIP, err := pcEndpoint.ParseIP()
	if err != nil {
		// If it's not able to parse IP correctly then, ignore the error as
		// we want to compare only IP addresses.
		return nil
	}

	if pcIP.Compare(controlPlaneVIP) == 0 {
		errMsg := fmt.Sprintf(
			"Prism Central and control plane endpoint cannot have the same IP %q",
			pcIP.String(),
		)
		return errors.New(errMsg)
	}

	return nil
}
