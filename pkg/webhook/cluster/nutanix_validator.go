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

	if err := validateTopologyFailureDomainConfig(clusterConfig, cluster); err != nil {
		return admission.Denied(err.Error())
	}

	return admission.Allowed("")
}

// validateTopologyFailureDomainConfig validates the failure domain related configuration in cluster topology.
func validateTopologyFailureDomainConfig(
	clusterConfig *variables.ClusterConfigSpec,
	cluster *clusterv1.Cluster,
) error {
	fldErrs := field.ErrorList{}

	// Validate control plane failure domain configuration
	if controlPlaneErrs := validateControlPlaneFailureDomainConfig(clusterConfig); controlPlaneErrs != nil {
		fldErrs = append(fldErrs, controlPlaneErrs...)
	}

	// Validate worker failure domain configuration
	if workerErrs := validateWorkerFailureDomainConfig(cluster); workerErrs != nil {
		fldErrs = append(fldErrs, workerErrs...)
	}

	return fldErrs.ToAggregate()
}

// validateControlPlaneFailureDomainConfig validates the failure domain related configuration for control plane.
func validateControlPlaneFailureDomainConfig(clusterConfig *variables.ClusterConfigSpec) field.ErrorList {
	var fldErrs field.ErrorList
	fldPath := field.NewPath("spec", "topology")

	if clusterConfig.ControlPlane == nil || clusterConfig.ControlPlane.Nutanix == nil {
		return fldErrs
	}

	machineDetails := clusterConfig.ControlPlane.Nutanix.MachineDetails
	failureDomainsConfigured := len(clusterConfig.ControlPlane.Nutanix.FailureDomains) > 0

	if failureDomainsConfigured {
		// When control plane failureDomains are configured, machineDetails must NOT have cluster/subnets
		if machineDetails.Cluster != nil {
			fldErrs = append(fldErrs, field.Forbidden(
				fldPath.Child(
					"variables",
					"clusterConfig",
					"value",
					"controlPlane",
					"nutanix",
					"machineDetails",
					"cluster",
				),
				"\"cluster\" must not be set when failureDomains are configured.",
			))
		}

		if len(machineDetails.Subnets) > 0 {
			fldErrs = append(fldErrs, field.Forbidden(
				fldPath.Child(
					"variables",
					"clusterConfig",
					"value",
					"controlPlane",
					"nutanix",
					"machineDetails",
					"subnets",
				),
				"\"subnets\" must not be set when failureDomains are configured.",
			))
		}
	} else {
		// When controlplane failureDomains are NOT configured, machineDetails MUST have cluster/subnets
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
				"\"cluster\" must be set when failureDomains are not configured.",
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
				"\"subnets\" must be set when failureDomains are not configured.",
			))
		}
	}

	return fldErrs
}

// validateWorkerFailureDomainConfig validates the failure domain related configuration for workers.
func validateWorkerFailureDomainConfig(cluster *clusterv1.Cluster) field.ErrorList {
	var fldErrs field.ErrorList
	fldPath := field.NewPath("spec", "topology")

	if cluster.Spec.Topology.Workers == nil {
		return fldErrs
	}

	// Get base workerConfig from global variables (if exists)
	baseWorkerConfig, err := variables.UnmarshalWorkerConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		fldErrs = append(fldErrs, field.InternalError(
			fldPath.Child("variables", "workerConfig"),
			fmt.Errorf("failed to unmarshal base worker topology variable: %w", err)))
	}

	for i := range cluster.Spec.Topology.Workers.MachineDeployments {
		md := cluster.Spec.Topology.Workers.MachineDeployments[i]
		failureDomainConfigured := md.FailureDomain != nil && *md.FailureDomain != ""

		// Handle machine deployments with explicit overrides
		if md.Variables != nil && len(md.Variables.Overrides) > 0 {
			if mdErrs := validateWorkerMachineDeploymentWithOverrides(md, baseWorkerConfig, failureDomainConfigured, fldPath); mdErrs != nil {
				fldErrs = append(fldErrs, mdErrs...)
			}
		} else if !failureDomainConfigured {
			// Handle machine deployments without overrides that rely entirely on base workerConfig
			if mdErrs := validateWorkerMachineDeploymentWithoutOverrides(baseWorkerConfig, fldPath); mdErrs != nil {
				fldErrs = append(fldErrs, mdErrs...)
			}
		}
	}

	return fldErrs
}

// validateWorkerMachineDeploymentWithOverrides validates a machine deployment that has variable overrides.
func validateWorkerMachineDeploymentWithOverrides(
	md clusterv1.MachineDeploymentTopology,
	baseWorkerConfig *variables.WorkerNodeConfigSpec,
	failureDomainConfigured bool,
	fldPath *field.Path,
) field.ErrorList {
	var fldErrs field.ErrorList

	overrideWorkerConfig, err := variables.UnmarshalWorkerConfigVariable(md.Variables.Overrides)
	if err != nil {
		fldErrs = append(fldErrs, field.InternalError(
			fldPath.Child("workers", "machineDeployments", "variables", "workerConfig"),
			fmt.Errorf("failed to unmarshal worker topology variable: %w", err)))
		return fldErrs
	}

	if overrideWorkerConfig.Nutanix == nil {
		return fldErrs
	}

	overrideMachineDetails := overrideWorkerConfig.Nutanix.MachineDetails

	if failureDomainConfigured {
		// When failureDomain is configured AND there are explicit variable overrides,
		// the overrides should NOT set cluster/subnets (they conflict with failure domain)
		if overrideMachineDetails.Cluster != nil {
			fldErrs = append(fldErrs, field.Forbidden(
				fldPath.Child(
					"workers",
					"machineDeployments",
					"variables",
					"workerConfig",
					"nutanix",
					"machineDetails",
					"cluster",
				),
				"\"cluster\" must not be set in variable overrides when failureDomain is configured.",
			))
		}
		if len(overrideMachineDetails.Subnets) > 0 {
			fldErrs = append(fldErrs, field.Forbidden(
				fldPath.Child(
					"workers",
					"machineDeployments",
					"variables",
					"workerConfig",
					"nutanix",
					"machineDetails",
					"subnets",
				),
				"\"subnets\" must not be set in variable overrides when failureDomain is configured.",
			))
		}
	} else {
		// When failureDomain is NOT configured, cluster/subnets must be available from either
		// the override variables OR the base workerConfig
		hasClusterInOverride := overrideMachineDetails.Cluster != nil &&
			(overrideMachineDetails.Cluster.IsName() || overrideMachineDetails.Cluster.IsUUID())
		hasClusterInBase := baseWorkerConfig != nil && baseWorkerConfig.Nutanix != nil &&
			baseWorkerConfig.Nutanix.MachineDetails.Cluster != nil &&
			(baseWorkerConfig.Nutanix.MachineDetails.Cluster.IsName() || baseWorkerConfig.Nutanix.MachineDetails.Cluster.IsUUID())

		if !hasClusterInOverride && !hasClusterInBase {
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
				"\"cluster\" must be set in either base workerConfig or variable overrides when failureDomain is not configured.",
			))
		}

		hasSubnetsInOverride := len(overrideMachineDetails.Subnets) > 0
		hasSubnetsInBase := baseWorkerConfig != nil && baseWorkerConfig.Nutanix != nil &&
			len(baseWorkerConfig.Nutanix.MachineDetails.Subnets) > 0

		if !hasSubnetsInOverride && !hasSubnetsInBase {
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
				"\"subnets\" must be set in either base workerConfig or variable overrides when failureDomain is not configured.",
			))
		}
	}

	return fldErrs
}

// validateWorkerMachineDeploymentWithoutOverrides validates a machine deployment that relies entirely on base workerConfig.
func validateWorkerMachineDeploymentWithoutOverrides(
	baseWorkerConfig *variables.WorkerNodeConfigSpec,
	fldPath *field.Path,
) field.ErrorList {
	var fldErrs field.ErrorList

	// Only validate if failureDomain is NOT configured
	if baseWorkerConfig == nil || baseWorkerConfig.Nutanix == nil {
		fldErrs = append(fldErrs, field.Required(
			fldPath.Child("variables", "workerConfig", "nutanix"),
			"base \"workerConfig\" must be configured when machine deployment has no failureDomain and no variable overrides.",
		))
		return fldErrs
	}

	baseMachineDetails := baseWorkerConfig.Nutanix.MachineDetails
	if baseMachineDetails.Cluster == nil ||
		!(baseMachineDetails.Cluster.IsName() || baseMachineDetails.Cluster.IsUUID()) {
		fldErrs = append(fldErrs, field.Required(
			fldPath.Child("variables", "workerConfig", "nutanix", "machineDetails", "cluster"),
			"\"cluster\" must be set in base workerConfig when machine deployment has no failureDomain and no variable overrides.",
		))
	}
	if len(baseMachineDetails.Subnets) == 0 {
		fldErrs = append(fldErrs, field.Required(
			fldPath.Child("variables", "workerConfig", "nutanix", "machineDetails", "subnets"),
			"\"subnets\" must be set in base workerConfig when machine deployment has no failureDomain and no variable overrides.",
		))
	}

	return fldErrs
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
