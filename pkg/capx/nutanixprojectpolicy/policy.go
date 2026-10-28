// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package nutanixprojectpolicy encodes CAPX/Nutanix project policy and CSI storage-project-policy.
package nutanixprojectpolicy

import (
	"strings"

	"github.com/nutanix-cloud-native/prism-go-client/versionutils"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

const (
	// NutanixProjectPolicyAnnotationKey is applied to cluster.x-k8s.io Cluster metadata for CAPX.
	NutanixProjectPolicyAnnotationKey = "capx.nutanix.com/project-policy"
	// NutanixProjectUUIDAnnotationKey pins a single-project cluster to one Prism project UUID.
	NutanixProjectUUIDAnnotationKey = "capx.nutanix.com/project-uuid"

	// PolicyDefaultOnly restricts deployments to the Prism default project.
	PolicyDefaultOnly = "default-only"
	// PolicyUnrestricted indicates no default-project-only restriction from this policy.
	PolicyUnrestricted = "unrestricted"
	// PolicySingleProject restricts a cluster's machine details to a single project, which may be a
	// non-default project.
	PolicySingleProject = "single-project"

	// CSIStorageProjectPolicyAnnotationKey records whether CSI and project-scoped AOS preflight
	// apply the new storage-project rules. Distinct from the CAPX tenancy annotation above.
	CSIStorageProjectPolicyAnnotationKey = "csi.nutanix.com/storage-project-policy"
	// StoragePolicyProjectPolicy is set on unpaused CREATE when the key is missing: CSI and
	// project-scoped AOS checks apply.
	StoragePolicyProjectPolicy = "project-policy"
	// StoragePolicyDefault is set on unpaused UPDATE when the annotation was never present, so
	// brownfield clusters skip those checks.
	StoragePolicyDefault = "default"
)

// StorageProjectPolicyIsDefault is true when the Cluster is grandfathered out of the CSI project
// and project-scoped AOS (PC 7.6 / AOS 7.5) preflight checks.
func StorageProjectPolicyIsDefault(annotations map[string]string) bool {
	return storageProjectPolicyEquals(annotations, StoragePolicyDefault)
}

// StorageProjectPolicyIsProjectPolicy is true when CSI and project-scoped AOS checks apply.
func StorageProjectPolicyIsProjectPolicy(annotations map[string]string) bool {
	return storageProjectPolicyEquals(annotations, StoragePolicyProjectPolicy)
}

func storageProjectPolicyEquals(annotations map[string]string, want string) bool {
	if len(annotations) == 0 {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(annotations[CSIStorageProjectPolicyAnnotationKey]), want)
}

// PCVersionAtLeast76 returns true when raw PC version (including internal labels) satisfies >= 7.6.0.
func PCVersionAtLeast76(pcVersionRaw string) bool {
	if pcVersionRaw == "" || strings.EqualFold(strings.TrimSpace(pcVersionRaw), "skipped") {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(pcVersionRaw))
	// Prism Central dev builds report a non-semver label; treat like a current PC for policy gating.
	if strings.Contains(lower, "master") {
		return true
	}
	parsed := versionutils.Parse(lower)
	if parsed.String() == "" {
		return false
	}
	return parsed.AtLeast(versionutils.Parse("7.6"))
}

// CollectReferencedProjects returns non-nil project identifiers from control plane and worker machine details.
func CollectReferencedProjects(
	clusterSpec *carenv1.NutanixClusterConfigSpec,
	workerSpecs map[string]*carenv1.NutanixWorkerNodeConfigSpec,
) []*capxv1.NutanixResourceIdentifier {
	var out []*capxv1.NutanixResourceIdentifier
	if clusterSpec != nil && clusterSpec.ControlPlane != nil && clusterSpec.ControlPlane.Nutanix != nil {
		if p := clusterSpec.ControlPlane.Nutanix.MachineDetails.Project; p != nil {
			out = append(out, p)
		}
	}
	for _, ws := range workerSpecs {
		if ws == nil || ws.Nutanix == nil {
			continue
		}
		if p := ws.Nutanix.MachineDetails.Project; p != nil {
			out = append(out, p)
		}
	}
	return out
}
