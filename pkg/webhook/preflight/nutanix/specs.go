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

type configurationCheck struct {
	result preflight.CheckResult
}

func (c *configurationCheck) Name() string {
	return "NutanixConfiguration"
}

func (c *configurationCheck) Run(_ context.Context) preflight.CheckResult {
	return c.result
}

func newConfigurationCheck(
	cd *checkDependencies,
) preflight.Check {
	cd.log.V(5).Info("Initializing Nutanix configuration check")

	configurationCheck := &configurationCheck{
		result: preflight.CheckResult{
			Allowed: true,
		},
	}

	nutanixClusterConfigSpec := &carenv1.NutanixClusterConfigSpec{}
	err := variables.UnmarshalClusterVariable(
		variables.GetClusterVariableByName(
			carenv1.ClusterConfigVariableName,
			cd.cluster.Spec.Topology.Variables,
		),
		nutanixClusterConfigSpec,
	)
	if err != nil {
		// Should not happen if the cluster passed CEL validation rules.
		configurationCheck.result.Allowed = false
		configurationCheck.result.InternalError = true
		configurationCheck.result.Causes = append(configurationCheck.result.Causes,
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
		cd.nutanixClusterConfigSpec = nutanixClusterConfigSpec
	}

	nutanixWorkerNodeConfigSpecByMachineDeploymentName := make(map[string]*carenv1.NutanixWorkerNodeConfigSpec)
	if cd.cluster.Spec.Topology.Workers != nil {
		for i := range cd.cluster.Spec.Topology.Workers.MachineDeployments {
			md := &cd.cluster.Spec.Topology.Workers.MachineDeployments[i]
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
				configurationCheck.result.Allowed = false
				configurationCheck.result.InternalError = true
				configurationCheck.result.Causes = append(configurationCheck.result.Causes,
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
		cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName = nutanixWorkerNodeConfigSpecByMachineDeploymentName
	}

	return configurationCheck
}
