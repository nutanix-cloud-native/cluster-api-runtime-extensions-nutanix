// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

func specsFromCluster(
	cluster *clusterv1.Cluster,
) (
	*carenv1.NutanixPrismCentralEndpointSpec,
	*carenv1.NutanixNodeSpec,
	map[string]*carenv1.NutanixNodeSpec,
	[]preflight.Cause,
) {
	var prismCentralEndpointSpec *carenv1.NutanixPrismCentralEndpointSpec
	var controlPlaneNutanixNodeSpec *carenv1.NutanixNodeSpec
	nutanixNodeSpecByMachineDeploymentName := make(map[string]*carenv1.NutanixNodeSpec)

	clusterConfig, err := variables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return nil, nil, nil, []preflight.Cause{
			{
				Message: err.Error(),
				Field:   "cluster.spec.topology.variables[.name=clusterConfig]",
			},
		}
	}

	if clusterConfig != nil && clusterConfig.Nutanix != nil {
		prismCentralEndpointSpec = &clusterConfig.Nutanix.PrismCentralEndpoint
	}

	if clusterConfig.ControlPlane != nil && clusterConfig.ControlPlane.Nutanix != nil {
		controlPlaneNutanixNodeSpec = clusterConfig.ControlPlane.Nutanix
	}

	causes := []preflight.Cause{}
	if cluster.Spec.Topology.Workers != nil {
		for _, md := range cluster.Spec.Topology.Workers.MachineDeployments {
			workerConfig, err := variables.UnmarshalWorkerConfigVariable(md.Variables.Overrides)
			if err != nil {
				causes = append(causes, preflight.Cause{
					Message: err.Error(),
					Field: fmt.Sprintf(
						"cluster.spec.topology.workers.machineDeployments[.name=%s].variables[.name=workerConfig]",
						md.Name,
					),
				})
			}

			if workerConfig != nil && workerConfig.Nutanix != nil {
				nutanixNodeSpecByMachineDeploymentName[md.Name] = workerConfig.Nutanix
			}
		}
	}
	return prismCentralEndpointSpec, controlPlaneNutanixNodeSpec, nutanixNodeSpecByMachineDeploymentName, causes
}
