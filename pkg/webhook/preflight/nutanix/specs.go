// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

func initNutanixConfiguration(
	n *nutanixChecker,
) preflight.Check {
	n.log.V(5).Info("Initializing Nutanix configuration check")

	result := preflight.CheckResult{
		Name:    "NutanixConfiguration",
		Allowed: true,
	}

	nutanixClusterConfigSpec := &carenv1.NutanixClusterConfigSpec{}
	err := variables.UnmarshalClusterVariable(
		variables.GetClusterVariableByName(
			carenv1.ClusterConfigVariableName,
			n.cluster.Spec.Topology.Variables,
		),
		nutanixClusterConfigSpec,
	)
	if err != nil {
		// Should not happen if the cluster passed CEL validation rules.
		result.Allowed = false
		result.Error = true
		result.Causes = append(result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf("Failed to unmarshal cluster variable %s: %s",
					carenv1.ClusterConfigVariableName,
					err,
				),
				Field: "cluster.spec.topology.variables[.name=clusterConfig].nutanix",
			},
		)
	}

	// Save the NutanixClusterConfigSpec only if it contains Nutanix configuration.
	if nutanixClusterConfigSpec.Nutanix != nil ||
		(nutanixClusterConfigSpec.ControlPlane != nil && nutanixClusterConfigSpec.ControlPlane.Nutanix != nil) {
		n.nutanixClusterConfigSpec = nutanixClusterConfigSpec
	}

	nutanixWorkerNodeConfigSpecByMachineDeploymentName := make(map[string]*carenv1.NutanixWorkerNodeConfigSpec)
	if n.cluster.Spec.Topology.Workers != nil {
		for _, md := range n.cluster.Spec.Topology.Workers.MachineDeployments {
			if md.Variables == nil {
				continue
			}
			nutanixWorkerNodeConfigSpec := &carenv1.NutanixWorkerNodeConfigSpec{}
			err := variables.UnmarshalClusterVariable(
				variables.GetClusterVariableByName(carenv1.WorkerConfigVariableName, md.Variables.Overrides),
				nutanixWorkerNodeConfigSpec,
			)
			if err != nil {
				// Should not happen if the cluster passed CEL validation rules.
				result.Allowed = false
				result.Error = true
				result.Causes = append(result.Causes,
					preflight.Cause{
						Message: fmt.Sprintf("Failed to unmarshal topology machineDeployment variable %s: %s",
							carenv1.WorkerConfigVariableName,
							err,
						),
						Field: fmt.Sprintf(
							"cluster.spec.topology.workers.machineDeployments[.name=%s]"+
								".variables[.name=workerConfig].value.nutanix.machineDetails",
							md.Name,
						),
					},
				)
			}
			// Save the NutanixWorkerNodeConfigSpec only if it contains Nutanix configuration.
			if nutanixWorkerNodeConfigSpec.Nutanix != nil {
				nutanixWorkerNodeConfigSpecByMachineDeploymentName[md.Name] = nutanixWorkerNodeConfigSpec
			}
		}
	}
	// Save the NutanixWorkerNodeConfigSpecByMachineDeploymentName only if it contains at least one Nutanix configuration.
	if len(nutanixWorkerNodeConfigSpecByMachineDeploymentName) > 0 {
		n.nutanixWorkerNodeConfigSpecByMachineDeploymentName = nutanixWorkerNodeConfigSpecByMachineDeploymentName
	}

	return func(ctx context.Context) preflight.CheckResult {
		return result
	}
}
