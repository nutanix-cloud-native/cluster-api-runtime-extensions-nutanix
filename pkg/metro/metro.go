// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package metro

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

const (
	// FailureDomainPrefix is the CAPI failure-domain name prefix for a NutanixMetro.
	FailureDomainPrefix = "NutanixMetro/"
	// SiteFailureDomainPrefix is the CAPI failure-domain name prefix for a NutanixMetroSite.
	SiteFailureDomainPrefix = "NutanixMetroSite/"

	// ComputeAffinityParameter is the Nutanix CSI StorageClass parameter that pins
	// a volume to the Prism Element hosting the consuming VM.
	ComputeAffinityParameter = "computeAffinity"
	// ComputeAffinityDisabled disables PE-local volume affinity. Metro clusters
	// require this value because they span two Prism Elements.
	ComputeAffinityDisabled = "DISABLED"
)

// IsMetroFailureDomain reports whether fd names a NutanixMetro or NutanixMetroSite.
func IsMetroFailureDomain(fd string) bool {
	return strings.HasPrefix(fd, FailureDomainPrefix) ||
		strings.HasPrefix(fd, SiteFailureDomainPrefix)
}

// IsMetroCluster returns true when the cluster uses metro-aware failure domains,
// i.e. any control-plane or worker failure domain references a NutanixMetro or
// NutanixMetroSite object (identified by the respective name prefix).
func IsMetroCluster(cluster *clusterv1.Cluster) bool {
	if cluster == nil || !cluster.Spec.Topology.IsDefined() {
		return false
	}

	clusterConfig, err := variables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err == nil &&
		clusterConfig != nil &&
		clusterConfig.ControlPlane != nil &&
		clusterConfig.ControlPlane.Nutanix != nil {
		if slices.ContainsFunc(clusterConfig.ControlPlane.Nutanix.FailureDomains, IsMetroFailureDomain) {
			return true
		}
	}

	for i := range cluster.Spec.Topology.Workers.MachineDeployments {
		if IsMetroFailureDomain(cluster.Spec.Topology.Workers.MachineDeployments[i].FailureDomain) {
			return true
		}
	}

	return false
}

// DefaultCSIComputeAffinity sets computeAffinity=DISABLED on Nutanix CSI
// StorageClassConfigs when the parameter is unset or empty. It does not overwrite
// a non-empty user-provided value.
//
// The caller is responsible for checking that the Cluster is a metro Cluster
// (see IsMetroCluster) before calling this function.
func DefaultCSIComputeAffinity(cluster *clusterv1.Cluster) error {
	clusterConfig, err := variables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return fmt.Errorf(
			"failed to unmarshal cluster topology variable %q: %w",
			v1alpha1.ClusterConfigVariableName,
			err,
		)
	}

	provider, ok := nutanixCSIProvider(clusterConfig)
	if !ok {
		return nil
	}

	for name, sc := range provider.StorageClassConfigs {
		if sc.Parameters == nil {
			sc.Parameters = map[string]string{}
		}
		if sc.Parameters[ComputeAffinityParameter] != "" {
			continue
		}
		sc.Parameters[ComputeAffinityParameter] = ComputeAffinityDisabled
		provider.StorageClassConfigs[name] = sc
	}

	clusterConfig.Addons.CSI.Providers[v1alpha1.CSIProviderNutanix] = *provider

	variable, err := variables.MarshalToClusterVariable(
		v1alpha1.ClusterConfigVariableName,
		clusterConfig,
	)
	if err != nil {
		return fmt.Errorf("failed to marshal cluster variable: %w", err)
	}
	cluster.Spec.Topology.Variables = variables.UpdateClusterVariable(
		variable,
		cluster.Spec.Topology.Variables,
	)
	return nil
}

// ValidateCSIComputeAffinity rejects a Cluster that sets computeAffinity to any
// value other than DISABLED.
//
// The caller is responsible for checking that the Cluster is a metro Cluster
// (see IsMetroCluster) before calling this function.
func ValidateCSIComputeAffinity(cluster *clusterv1.Cluster) error {
	clusterConfig, err := variables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return fmt.Errorf(
			"failed to unmarshal cluster topology variable %q: %w",
			v1alpha1.ClusterConfigVariableName,
			err,
		)
	}

	provider, ok := nutanixCSIProvider(clusterConfig)
	if !ok {
		return nil
	}

	var errs []error
	for name, sc := range provider.StorageClassConfigs {
		value := ""
		if sc.Parameters != nil {
			value = sc.Parameters[ComputeAffinityParameter]
		}
		if value == "" || value == ComputeAffinityDisabled {
			continue
		}
		errs = append(errs, fmt.Errorf(
			"computeAffinity %q on storage class %q is not supported for the metro cluster",
			value,
			name,
		))
	}
	return errors.Join(errs...)
}

func nutanixCSIProvider(clusterConfig *variables.ClusterConfigSpec) (*v1alpha1.CSIProvider, bool) {
	if clusterConfig == nil ||
		clusterConfig.Addons == nil ||
		clusterConfig.Addons.CSI == nil ||
		clusterConfig.Addons.CSI.Providers == nil {
		return nil, false
	}
	provider, ok := clusterConfig.Addons.CSI.Providers[v1alpha1.CSIProviderNutanix]
	if !ok {
		return nil, false
	}
	return &provider, true
}
