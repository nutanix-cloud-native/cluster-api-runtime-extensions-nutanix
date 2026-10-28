// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/capx/nutanixprojectpolicy"
)

const (
	// clusterProjectPolicyField points at the Cluster annotation that carries the project policy.
	// Shared by the metro and CSI project checks.
	clusterProjectPolicyField = "$.metadata.annotations['" +
		nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey + "']"

	// zeroProjectUUID is a stand-in for the default project used when the real default
	// project UUID cannot be resolved (e.g. a project-scoped user on PC 7.6+ without
	// permission to read the default project).
	zeroProjectUUID = "00000000-0000-0000-0000-000000000000"

	// controlPlaneMachineDetailsField is the JSONPath to the control plane's machineDetails value within
	// clusterConfig. Shared by every check that validates a resource referenced directly by the control
	// plane's Nutanix machine details (not via a failure domain).
	controlPlaneMachineDetailsField = "$.spec.topology.variables[?@.name==\"clusterConfig\"].value.controlPlane.nutanix.machineDetails" //nolint:lll // Field is long.
)
