// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/capx/nutanixprojectpolicy"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

const (
	// hypervisorAttachedParameterKey is the Nutanix CSI StorageClass parameter that controls whether
	// volumes are attached through the AHV hypervisor. It is an enum: "ENABLED" (hypervisor-attached) or
	// "DISABLED" (guest/direct-iSCSI attached, e.g. for bare-metal nodes). When the parameter is absent,
	// the Nutanix CSI driver defaults to DISABLED (iSCSI) mode. Only hypervisor-attached (ENABLED) volumes
	// support Nutanix project placement, so a StorageClass supports projects only when it explicitly sets
	// hypervisorAttached=ENABLED.
	hypervisorAttachedParameterKey = "hypervisorAttached"
	hypervisorAttachedEnabledValue = "ENABLED"

	// nutanixCSIProjectPolicyName is the name of the check that ensures a Cluster using
	// non-hypervisor-attached volumes is restricted to the default Prism Central project policy.
	nutanixCSIProjectPolicyName = "NutanixCSIProjectPolicy"
	// nutanixCSIProjectReferenceName is the name of the check that ensures a Cluster using
	// non-hypervisor-attached volumes only references the default Prism Central project.
	nutanixCSIProjectReferenceName = "NutanixCSIProjectReference"
)

// newCSIChecks returns the checks that enforce that a Cluster configuring a Nutanix CSI StorageClass with
// hypervisorAttached=DISABLED does not use a Prism Central project other than the default one. Nutanix
// project placement currently requires hypervisor-attached volumes, so a cluster with
// non-hypervisor-attached volumes must use the default-only project policy and must not reference a
// non-default project.
//
// The pair of checks is only added when the gating condition holds: at least one StorageClass is not
// explicitly hypervisorAttached=ENABLED (an absent parameter defaults to DISABLED/iSCSI). Clusters
// annotated csi.nutanix.com/storage-project-policy=default are brownfield and skip these checks.
func newCSIChecks(cd *checkDependencies) []preflight.Check {
	checks := make([]preflight.Check, 0, 2)

	if cd == nil || cd.cluster == nil || cd.nutanixClusterConfigSpec == nil {
		return checks
	}
	if nutanixprojectpolicy.StorageProjectPolicyIsDefault(cd.cluster.Annotations) {
		return checks
	}

	scNames := csiStorageClassConfigsWithNonHypervisorAttachedVolumes(cd)
	if len(scNames) == 0 {
		return checks
	}

	policy := ""
	if cd.cluster.Annotations != nil {
		policy = strings.TrimSpace(cd.cluster.Annotations[nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey])
	}

	checks = append(checks,
		&csiProjectPolicyCheck{
			policy:                 policy,
			storageClassConfigName: scNames,
			field:                  clusterProjectPolicyField,
		},
		&csiProjectReferenceCheck{
			refs:                   collectNutanixProjectRefsWithFields(cd),
			nclient:                cd.nclient,
			pcVersion:              cd.pcVersion,
			storageClassConfigName: scNames,
			field:                  clusterProjectPolicyField,
		},
	)

	return checks
}

// csiStorageClassConfigsWithNonHypervisorAttachedVolumes returns the sorted names of the Nutanix CSI
// StorageClass configs that are not explicitly hypervisorAttached=ENABLED. The Nutanix CSI driver
// defaults an absent parameter to DISABLED (iSCSI), so anything other than an explicit ENABLED is treated
// as non-hypervisor-attached and therefore unable to use Nutanix project placement.
func csiStorageClassConfigsWithNonHypervisorAttachedVolumes(cd *checkDependencies) []string {
	spec := cd.nutanixClusterConfigSpec
	if spec == nil || spec.Addons == nil || spec.Addons.CSI == nil {
		return nil
	}
	configs := spec.Addons.CSI.Providers.NutanixCSI.StorageClassConfigs
	if len(configs) == 0 {
		return nil
	}

	var names []string
	for name, cfg := range configs {
		// The Nutanix CSI driver operates in iSCSI (non-hypervisor-attached) mode unless the StorageClass
		// explicitly sets hypervisorAttached=ENABLED; an absent parameter defaults to DISABLED. Treat
		// anything other than an explicit ENABLED as non-hypervisor-attached.
		value, ok := cfg.Parameters[hypervisorAttachedParameterKey]
		if ok && strings.EqualFold(strings.TrimSpace(value), hypervisorAttachedEnabledValue) {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// csiProjectPolicyCheck enforces that a Cluster using non-hypervisor-attached volumes has its project
// policy restricted to the default Prism Central project (default-only).
type csiProjectPolicyCheck struct {
	policy                 string
	storageClassConfigName []string
	field                  string
}

func (c *csiProjectPolicyCheck) Name() string {
	return nutanixCSIProjectPolicyName
}

func (c *csiProjectPolicyCheck) Run(_ context.Context) preflight.CheckResult {
	result := preflight.CheckResult{Allowed: true}

	if c.policy != nutanixprojectpolicy.PolicyDefaultOnly {
		failCheck(&result, c.field, fmt.Sprintf(
			"Project Policy Violation: A cluster using non-hypervisor-attached volumes is only supported "+
				"with the default Prism Central project. To use a non-default Prism Central project, configure "+
				"the StorageClass (%s) with %s=%s.",
			strings.Join(c.storageClassConfigName, ", "),
			hypervisorAttachedParameterKey,
			hypervisorAttachedEnabledValue,
		))
	}

	return result
}

// csiProjectReferenceCheck enforces that a Cluster using non-hypervisor-attached volumes does not
// reference a non-default Prism Central project in any machine details.
type csiProjectReferenceCheck struct {
	refs                   []projectRefWithField
	nclient                client
	pcVersion              string
	storageClassConfigName []string
	field                  string
}

func (c *csiProjectReferenceCheck) Name() string {
	return nutanixCSIProjectReferenceName
}

func (c *csiProjectReferenceCheck) Run(ctx context.Context) preflight.CheckResult {
	result := preflight.CheckResult{Allowed: true}

	if len(c.refs) == 0 {
		return result
	}

	if c.nclient == nil {
		failCheckClientNotInitialized(
			&result,
			c.field,
			"validate the default project for the cluster using non-hypervisor-attached volumes",
		)
		return result
	}

	def, err := c.nclient.ResolvePCDefaultProject(ctx, c.pcVersion)
	if err != nil {
		// Fall back to the zero-UUID default project when the real default project cannot be
		// resolved (e.g. a project-scoped user without permission to read it).
		def = &PCDefaultProject{UUID: zeroProjectUUID}
	}

	for _, ref := range c.refs {
		if ProjectMatchesDefault(ref.id, def) {
			continue
		}
		failCheck(&result, ref.field, fmt.Sprintf(
			"A cluster with non-hypervisor-attached volumes can only use the default Prism Central project, "+
				"but a non-default project is referenced in the machine details. Set %s=%s in the "+
				"StorageClass parameters (%s) to use a non-default project.",
			hypervisorAttachedParameterKey,
			hypervisorAttachedEnabledValue,
			strings.Join(c.storageClassConfigName, ", "),
		))
	}

	return result
}
