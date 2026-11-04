// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"strings"

	v1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	clusterctlv1 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/capx/nutanixprojectpolicy"
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

	if !cluster.Spec.Topology.IsDefined() {
		return admission.Allowed("")
	}

	if utils.GetProvider(cluster) != "nutanix" {
		return admission.Allowed("")
	}

	// The Nutanix project policy (and, for single-project, the pinned project UUID) may only be
	// narrowed, never widened, once set. A cluster's project is an irreversible tenant boundary, so
	// leaving default-only or single-project, or swapping the project UUID, would let new machines
	// land outside that boundary.
	if req.Operation == v1.Update {
		oldCluster := &clusterv1.Cluster{}
		if err := a.decoder.DecodeRaw(req.OldObject, oldCluster); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if projectPolicyAnnotationWidened(oldCluster, cluster) {
			key := nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey
			return admission.Denied(fmt.Sprintf(
				"the %q annotation cannot be changed from %q to %q: the Nutanix project policy may only be "+
					"narrowed (a cluster's project is an irreversible tenant boundary), not widened",
				key,
				oldCluster.Annotations[key],
				annotationValueForMessage(cluster.Annotations[key]),
			))
		}
		if projectUUIDAnnotationChanged(oldCluster, cluster) {
			key := nutanixprojectpolicy.NutanixProjectUUIDAnnotationKey
			return admission.Denied(fmt.Sprintf(
				"project UUID is immutable and cannot be changed for clusters obeying single-project policy "+
					"(from %q to %q)",
				oldCluster.Annotations[key],
				annotationValueForMessage(cluster.Annotations[key]),
			))
		}
		if storageProjectPolicyWidened(oldCluster, cluster) {
			key := nutanixprojectpolicy.CSIStorageProjectPolicyAnnotationKey
			return admission.Denied(fmt.Sprintf(
				"the %q annotation cannot be changed from %q to %q: CSI storage project policy may only be "+
					"narrowed (from default to project-policy), not widened",
				key,
				oldCluster.Annotations[key],
				annotationValueForMessage(cluster.Annotations[key]),
			))
		}
	}

	clusterConfig, err := variables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return admission.Denied(
			fmt.Errorf("failed to unmarshal cluster topology variable %q: %w",
				v1alpha1.ClusterConfigVariableName,
				err).Error(),
		)
	}
	// A Cluster without the clusterConfig variable has nothing for this validator to check.
	if clusterConfig == nil {
		return admission.Allowed("")
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

// validateFailureDomainXORMachineDetails validates the XOR behavior between failure domains
// and cluster/subnets configuration. Either failure domains are set OR cluster and subnets
// are set with machineDetails, but not both.
func validateFailureDomainXORMachineDetails(
	fldPath *field.Path,
	hasFailureDomains bool,
	machineDetails *v1alpha1.NutanixMachineDetails,
	failureDomainTerm string, // "failureDomains" for control plane, "failureDomain" for workers
) field.ErrorList {
	fldErrs := field.ErrorList{}

	// Determine the correct verb based on singular/plural
	verb := "is"
	if failureDomainTerm == "failureDomains" {
		verb = "are"
	}

	hasCluster, hasSubnets := machineDetails.HasClusterAndSubnets()

	if !hasFailureDomains {
		// No failure domains -> cluster/subnets MUST be set
		if !hasCluster {
			fldErrs = append(fldErrs, field.Required(
				fldPath.Child("cluster"),
				fmt.Sprintf("\"cluster\" must be set when %s %s not configured.", failureDomainTerm, verb),
			))
		}

		if !hasSubnets {
			fldErrs = append(fldErrs, field.Required(
				fldPath.Child("subnets"),
				fmt.Sprintf("\"subnets\" must be set when %s %s not configured.", failureDomainTerm, verb),
			))
		}
	} else {
		// Failure domains present -> cluster/subnets MUST NOT be set
		if hasCluster {
			fldErrs = append(fldErrs, field.Forbidden(
				fldPath.Child("cluster"),
				fmt.Sprintf("\"cluster\" must not be set when %s %s configured.", failureDomainTerm, verb),
			))
		}

		if hasSubnets {
			fldErrs = append(fldErrs, field.Forbidden(
				fldPath.Child("subnets"),
				fmt.Sprintf("\"subnets\" must not be set when %s %s configured.", failureDomainTerm, verb),
			))
		}
	}

	return fldErrs
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

// validateControlPlaneFailureDomainConfig validates XOR behavior: either failureDomains are set
// OR cluster and subnets are set with machineDetails, but not both, for control plane.
func validateControlPlaneFailureDomainConfig(clusterConfig *variables.ClusterConfigSpec) field.ErrorList {
	fldErrs := field.ErrorList{}
	fldPath := field.NewPath(
		"spec",
		"topology",
		"variables",
		"clusterConfig",
		"value",
		"controlPlane",
		"nutanix",
		"machineDetails",
	)

	if clusterConfig.ControlPlane != nil && clusterConfig.ControlPlane.Nutanix != nil {
		machineDetails := clusterConfig.ControlPlane.Nutanix.MachineDetails
		hasFailureDomains := len(clusterConfig.ControlPlane.Nutanix.FailureDomains) > 0

		fldErrs = append(
			fldErrs,
			validateFailureDomainXORMachineDetails(
				fldPath,
				hasFailureDomains,
				&machineDetails,
				"failureDomains",
			)...,
		)
	}

	return fldErrs
}

// validateWorkerFailureDomainConfig validates XOR behavior: either failureDomain is set
// OR cluster and subnets are set with machineDetails, but not both, for workers.
func validateWorkerFailureDomainConfig(
	cluster *clusterv1.Cluster,
) field.ErrorList {
	fldErrs := field.ErrorList{}
	workerConfigVarPath := field.NewPath("spec", "topology", "variables", "workerConfig")
	workerConfigMDVarOverridePath := field.NewPath(
		"spec",
		"topology",
		"workers",
		"machineDeployments",
		"variables",
		"overrides",
		"workerConfig",
	)

	// Get the machineDetails from cluster variable "workerConfig" if it is configured
	defaultWorkerConfig, err := variables.UnmarshalWorkerConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		fldErrs = append(fldErrs, field.InternalError(workerConfigVarPath,
			fmt.Errorf("failed to unmarshal cluster topology variable %q: %w", v1alpha1.WorkerConfigVariableName, err)))
	}

	if len(cluster.Spec.Topology.Workers.MachineDeployments) > 0 {
		for i := range cluster.Spec.Topology.Workers.MachineDeployments {
			md := cluster.Spec.Topology.Workers.MachineDeployments[i]
			hasFailureDomain := md.FailureDomain != ""

			// Get the machineDetails from the overrides variable "workerConfig" if it is configured,
			// otherwise use the defaultWorkerConfig if it is configured.
			var workerConfig *variables.WorkerNodeConfigSpec
			if len(md.Variables.Overrides) > 0 {
				workerConfig, err = variables.UnmarshalWorkerConfigVariable(md.Variables.Overrides)
				if err != nil {
					fldErrs = append(fldErrs, field.InternalError(
						workerConfigMDVarOverridePath,
						fmt.Errorf(
							"failed to unmarshal worker overrides variable %q: %w",
							v1alpha1.WorkerConfigVariableName,
							err,
						),
					))
				}
			}

			wcfgPath := workerConfigMDVarOverridePath
			if workerConfig == nil {
				workerConfig = defaultWorkerConfig
				wcfgPath = workerConfigVarPath
			}
			if workerConfig == nil || workerConfig.Nutanix == nil {
				continue
			}

			machineDetails := workerConfig.Nutanix.MachineDetails

			fldErrs = append(
				fldErrs,
				validateFailureDomainXORMachineDetails(
					wcfgPath.Child("value", "nutanix", "machineDetails"),
					hasFailureDomain,
					&machineDetails,
					"failureDomain",
				)...,
			)
		}
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

// projectPolicyAnnotationWidened reports whether the project policy annotation is being loosened.
// default-only and single-project are committed tenant boundaries, so any transition away from them
// (to unrestricted, the other frozen policy, absent, or unknown) is a widening.
func projectPolicyAnnotationWidened(oldCluster, newCluster *clusterv1.Cluster) bool {
	key := nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey

	// clusterctl move stamps the delete-for-move annotation on each source object (right before deleting
	// it) via a JSON Patch that replaces the entire /metadata/annotations map, which necessarily drops the
	// project-policy annotation. That paused, about-to-be-deleted source object is not a genuine widening,
	// so blocking the transition only breaks the pivot (NCN-115866). Let it through.
	if _, moving := newCluster.Annotations[clusterctlv1.DeleteForMoveAnnotation]; moving {
		return false
	}

	oldPolicy := oldCluster.Annotations[key]
	newPolicy := newCluster.Annotations[key]
	if oldPolicy == newPolicy {
		return false
	}

	return oldPolicy == nutanixprojectpolicy.PolicyDefaultOnly ||
		oldPolicy == nutanixprojectpolicy.PolicySingleProject
}

// projectUUIDAnnotationChanged reports whether a committed single-project UUID is being changed.
// Once a cluster is pinned to a project UUID, swapping or removing that UUID would let new machines
// land in a different tenant. Stamping a UUID onto a single-project cluster that did not yet have
// one is allowed (it only narrows).
func projectUUIDAnnotationChanged(oldCluster, newCluster *clusterv1.Cluster) bool {
	if _, moving := newCluster.Annotations[clusterctlv1.DeleteForMoveAnnotation]; moving {
		return false
	}

	if oldCluster.Annotations[nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey] !=
		nutanixprojectpolicy.PolicySingleProject {
		return false
	}

	oldUUID := strings.TrimSpace(oldCluster.Annotations[nutanixprojectpolicy.NutanixProjectUUIDAnnotationKey])
	if oldUUID == "" {
		return false
	}

	newUUID := strings.TrimSpace(newCluster.Annotations[nutanixprojectpolicy.NutanixProjectUUIDAnnotationKey])
	return !strings.EqualFold(oldUUID, newUUID)
}

// storageProjectPolicyWidened reports whether csi.nutanix.com/storage-project-policy is being loosened
// away from project-policy (to default, unset, or any other value). default → project-policy is a
// narrowing and is allowed.
func storageProjectPolicyWidened(oldCluster, newCluster *clusterv1.Cluster) bool {
	if _, moving := newCluster.Annotations[clusterctlv1.DeleteForMoveAnnotation]; moving {
		return false
	}
	if !nutanixprojectpolicy.StorageProjectPolicyIsProjectPolicy(oldCluster.Annotations) {
		return false
	}
	return !nutanixprojectpolicy.StorageProjectPolicyIsProjectPolicy(newCluster.Annotations)
}

func annotationValueForMessage(value string) string {
	if value == "" {
		return "<unset>"
	}
	return value
}
