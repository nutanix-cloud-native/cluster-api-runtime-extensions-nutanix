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
				Message: fmt.Sprintf(
					"Failed to unmarshal cluster variable %q: %s. Review the Cluster.", ///nolint:lll // Message is long.
					carenv1.ClusterConfigVariableName,
					err,
				),
				Field: "$.spec.topology.variables[?@.name==\"clusterConfig\"].value.nutanix",
			},
		)
	}

	// Save the NutanixClusterConfigSpec only if it contains Nutanix configuration.
	if nutanixClusterConfigSpec.Nutanix != nil ||
		(nutanixClusterConfigSpec.ControlPlane != nil && nutanixClusterConfigSpec.ControlPlane.Nutanix != nil) {
		cd.nutanixClusterConfigSpec = nutanixClusterConfigSpec
	}

	failureDomainByMachineDeploymentName := make(map[string]string)
	nutanixWorkerNodeConfigSpecByMachineDeploymentName := make(map[string]*carenv1.NutanixWorkerNodeConfigSpec)

	if cd.cluster.Spec.Topology.Workers != nil {
		for i := range cd.cluster.Spec.Topology.Workers.MachineDeployments {
			md := &cd.cluster.Spec.Topology.Workers.MachineDeployments[i]

			// Save the failureDomain only if it is configured
			if md.FailureDomain != nil && *md.FailureDomain != "" {
				failureDomainByMachineDeploymentName[md.Name] = *md.FailureDomain
			}

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
						Message: fmt.Sprintf(
							"Failed to unmarshal topology machineDeployment variable %q: %s. Review the Cluster.", ///nolint:lll // Message is long.
							carenv1.WorkerConfigVariableName,
							err,
						),
						//nolint:lll // The field is long.
						Field: fmt.Sprintf(
							"$.spec.topology.workers.machineDeployments[?@.name==%q].variables[?@.name=workerConfig].value.nutanix.machineDetails",
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

	// Save the failureDomainByMachineDeploymentName
	cd.failureDomainByMachineDeploymentName = failureDomainByMachineDeploymentName

	return configurationCheck
}
