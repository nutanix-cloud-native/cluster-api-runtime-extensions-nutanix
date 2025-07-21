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

	// Check if control plane has failure domains (calculate once for both validations)
	hasControlPlaneFailureDomains := clusterConfig.ControlPlane != nil &&
		clusterConfig.ControlPlane.Nutanix != nil &&
		len(clusterConfig.ControlPlane.Nutanix.FailureDomains) > 0

	// Validate control plane failure domain configuration
	if controlPlaneErrs := validateControlPlaneFailureDomainConfig(
		clusterConfig, hasControlPlaneFailureDomains); controlPlaneErrs != nil {
		fldErrs = append(fldErrs, controlPlaneErrs...)
	}

	// Validate worker failure domain configuration
	if workerErrs := validateWorkerFailureDomainConfig(cluster, hasControlPlaneFailureDomains); workerErrs != nil {
		fldErrs = append(fldErrs, workerErrs...)
	}

	return fldErrs.ToAggregate()
}

// validateControlPlaneFailureDomainConfig validates XOR logic between failureDomains and
// cluster/subnets configuration for control plane.
func validateControlPlaneFailureDomainConfig(
	clusterConfig *variables.ClusterConfigSpec,
	hasFailureDomains bool,
) field.ErrorList {
	if clusterConfig.ControlPlane == nil || clusterConfig.ControlPlane.Nutanix == nil {
		return nil
	}

	machineDetails := clusterConfig.ControlPlane.Nutanix.MachineDetails
	basePath := field.NewPath(
		"spec", "topology", "variables", "clusterConfig", "value", "controlPlane", "nutanix", "machineDetails",
	)

	if !hasFailureDomains {
		// Case 1: No failureDomains - cluster and subnets MUST be set
		return validateMachineDetailsRequiredWithMessages(&machineDetails, basePath,
			"'cluster' must be set when failureDomains are not configured.",
			"'subnets' must be set when failureDomains are not configured.")
	}

	// Case 2: failureDomains are present - cluster and subnets MUST NOT be set
	return validateMachineDetailsNotSetWithMessages(&machineDetails, basePath,
		"'cluster' must not be set when failureDomains are configured.",
		"'subnets' must not be set when failureDomains are configured.")
}

// validateWorkerFailureDomainConfig validates XOR logic between failureDomains and
// cluster/subnets configuration for workers.
func validateWorkerFailureDomainConfig(cluster *clusterv1.Cluster, hasControlPlaneFailureDomains bool) field.ErrorList {
	fldErrs := field.ErrorList{}

	// Get default worker config
	defaultWorkerConfig, err := variables.UnmarshalWorkerConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		fldErrs = append(fldErrs, field.InternalError(
			field.NewPath("spec", "topology", "variables", "workerConfig"),
			fmt.Errorf("failed to unmarshal cluster topology variable %q: %w", v1alpha1.WorkerConfigVariableName, err)))
	}

	// Cross-validation: control plane failure domains vs default worker config
	if err := validateDefaultWorkerConfigFailureDomains(hasControlPlaneFailureDomains, defaultWorkerConfig); err != nil {
		fldErrs = append(fldErrs, err...)
	}

	// Validate each machine deployment
	if cluster.Spec.Topology.Workers != nil {
		for i := range cluster.Spec.Topology.Workers.MachineDeployments {
			md := &cluster.Spec.Topology.Workers.MachineDeployments[i]
			if mdErrs := validateMachineDeploymentFailureDomainConfig(md, defaultWorkerConfig); mdErrs != nil {
				fldErrs = append(fldErrs, mdErrs...)
			}
		}
	}

	return fldErrs
}

// validateDefaultWorkerConfigFailureDomains validates that if control plane has failure domains,
// the default worker config should not have cluster/subnets configured.
func validateDefaultWorkerConfigFailureDomains(
	hasControlPlaneFailureDomains bool,
	defaultWorkerConfig *variables.WorkerNodeConfigSpec,
) field.ErrorList {
	if !hasControlPlaneFailureDomains || defaultWorkerConfig == nil || defaultWorkerConfig.Nutanix == nil {
		return nil
	}

	basePath := field.NewPath("spec", "topology", "variables", "workerConfig", "value", "nutanix", "machineDetails")
	return validateMachineDetailsNotSetWithMessages(&defaultWorkerConfig.Nutanix.MachineDetails, basePath,
		"'cluster' must not be set in default workerConfig when control plane has failureDomains configured.",
		"'subnets' must not be set in default workerConfig when control plane has failureDomains configured.")
}

// validateMachineDeploymentFailureDomainConfig validates XOR logic for a single machine deployment.
func validateMachineDeploymentFailureDomainConfig(
	md *clusterv1.MachineDeploymentTopology,
	defaultWorkerConfig *variables.WorkerNodeConfigSpec,
) field.ErrorList {
	fldErrs := field.ErrorList{}
	hasFailureDomain := md.FailureDomain != nil && *md.FailureDomain != ""

	// Determine which worker config to use and its field path
	workerConfig, configPath := resolveWorkerConfig(md, defaultWorkerConfig)

	// Case 1: No failure domain configured
	if !hasFailureDomain {
		if workerConfig == nil {
			// No worker config at all - this is an error
			fldErrs = append(fldErrs,
				field.Required(
					configPath.Child("cluster"),
					"'cluster' must be set when failureDomain is not configured.",
				),
				field.Required(
					configPath.Child("subnets"),
					"'subnets' must be set when failureDomain is not configured.",
				),
			)
			return fldErrs
		}

		if workerConfig.Nutanix != nil {
			// Validate that cluster and subnets ARE set
			if reqErrs := validateMachineDetailsRequired(&workerConfig.Nutanix.MachineDetails, configPath); reqErrs != nil {
				fldErrs = append(fldErrs, reqErrs...)
			}
		}
		return fldErrs
	}

	// Case 2: Failure domain is configured
	if workerConfig != nil && workerConfig.Nutanix != nil {
		// Validate that cluster and subnets are NOT set
		if forbiddenErrs := validateMachineDetailsNotSetWithMessages(&workerConfig.Nutanix.MachineDetails, configPath,
			"'cluster' must not be set when failureDomain is configured.",
			"'subnets' must not be set when failureDomain is configured."); forbiddenErrs != nil {
			fldErrs = append(fldErrs, forbiddenErrs...)
		}
	}

	return fldErrs
}

// resolveWorkerConfig determines which worker config to use and returns the appropriate field path.
func resolveWorkerConfig(
	md *clusterv1.MachineDeploymentTopology,
	defaultWorkerConfig *variables.WorkerNodeConfigSpec,
) (*variables.WorkerNodeConfigSpec, *field.Path) {
	// Try override config first
	if md.Variables != nil && len(md.Variables.Overrides) > 0 {
		if overrideConfig, err := variables.UnmarshalWorkerConfigVariable(md.Variables.Overrides); err == nil &&
			overrideConfig != nil {
			return overrideConfig, field.NewPath(
				"spec",
				"topology",
				"workers",
				"machineDeployments",
				"variables",
				"overrides",
				"workerConfig",
				"value",
				"nutanix",
				"machineDetails",
			)
		}
	}

	// Fall back to default config
	return defaultWorkerConfig, field.NewPath(
		"spec",
		"topology",
		"variables",
		"workerConfig",
		"value",
		"nutanix",
		"machineDetails",
	)
}

// validateMachineDetailsRequired validates that cluster and subnets are configured.
func validateMachineDetailsRequired(
	machineDetails *v1alpha1.NutanixMachineDetails,
	basePath *field.Path,
) field.ErrorList {
	return validateMachineDetailsRequiredWithMessages(machineDetails, basePath,
		"'cluster' must be set when failureDomain is not configured.",
		"'subnets' must be set when failureDomain is not configured.")
}

// validateMachineDetailsRequiredWithMessages validates that cluster and subnets are configured with custom messages.
func validateMachineDetailsRequiredWithMessages(
	machineDetails *v1alpha1.NutanixMachineDetails,
	basePath *field.Path,
	clusterMsg, subnetsMsg string,
) field.ErrorList {
	fldErrs := field.ErrorList{}
	hasCluster, hasSubnets := checkClusterAndSubnetPresence(machineDetails)

	if !hasCluster {
		fldErrs = append(fldErrs, field.Required(basePath.Child("cluster"), clusterMsg))
	}
	if !hasSubnets {
		fldErrs = append(fldErrs, field.Required(basePath.Child("subnets"), subnetsMsg))
	}
	return fldErrs
}

// validateMachineDetailsNotSetWithMessages validates that cluster and subnets are NOT configured.
func validateMachineDetailsNotSetWithMessages(
	machineDetails *v1alpha1.NutanixMachineDetails,
	basePath *field.Path,
	clusterMsg, subnetsMsg string,
) field.ErrorList {
	fldErrs := field.ErrorList{}
	hasCluster, hasSubnets := checkClusterAndSubnetPresence(machineDetails)

	if hasCluster {
		fldErrs = append(fldErrs, field.Forbidden(basePath.Child("cluster"), clusterMsg))
	}
	if hasSubnets {
		fldErrs = append(fldErrs, field.Forbidden(basePath.Child("subnets"), subnetsMsg))
	}
	return fldErrs
}

// checkClusterAndSubnetPresence checks if cluster and subnets are configured in machine details.
func checkClusterAndSubnetPresence(machineDetails *v1alpha1.NutanixMachineDetails) (hasCluster, hasSubnets bool) {
	hasCluster = machineDetails.Cluster != nil && (machineDetails.Cluster.IsName() || machineDetails.Cluster.IsUUID())
	hasSubnets = len(machineDetails.Subnets) > 0
	return hasCluster, hasSubnets
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
