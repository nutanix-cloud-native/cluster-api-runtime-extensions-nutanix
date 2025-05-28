// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	prismv4 "github.com/nutanix-cloud-native/prism-go-client/v4"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

type Checker struct {
	client                                 ctrlclient.Client
	cluster                                *clusterv1.Cluster
	nutanixSpec                            *carenv1.NutanixSpec
	nutanixControlPlaneNodeSpec            *carenv1.NutanixNodeSpec
	nutanixNodeSpecByMachineDeploymentName map[string]*carenv1.NutanixNodeSpec
	v4client                               *prismv4.Client
}

func (n *Checker) Init(
	ctx context.Context,
	client ctrlclient.Client,
	cluster *clusterv1.Cluster,
) []preflight.Check {
	n.client = client
	n.cluster = cluster

	var err error

	n.nutanixSpec, n.nutanixControlPlaneNodeSpec, n.nutanixNodeSpecByMachineDeploymentName, err = getData(cluster)
	if err != nil {
		return []preflight.Check{
			errorCheck("TBD", "TBD"),
		}
	}

	credentials, err := getCredentials(ctx, client, cluster, n.nutanixSpec)
	if err != nil {
		return []preflight.Check{
			errorCheck(
				fmt.Sprintf("failed to get Nutanix credentials: %s", err),
				"cluster.spec.topology.variables[.name=clusterConfig].nutanix.prismCentralEndpoint.credentials",
			),
		}
	}

	n.v4client, err = v4client(credentials, n.nutanixSpec)
	if err != nil {
		return []preflight.Check{
			errorCheck(
				fmt.Sprintf("failed to initialize Nutanix v4 client: %s", err),
				"cluster.spec.topology.variables[.name=clusterConfig].nutanix",
			),
		}
	}

	// Initialize checks.
	checks := []preflight.Check{}
	if n.nutanixControlPlaneNodeSpec != nil {
		checks = append(checks,
			func(ctx context.Context) preflight.CheckResult {
				return n.vmImageCheckForMachineDetails(
					&n.nutanixControlPlaneNodeSpec.MachineDetails,
					"cluster.spec.topology[.name=clusterConfig].value.controlPlane.nutanix.machineDetails",
				)
			},
		)
	}
	for mdName, nodeSpec := range n.nutanixNodeSpecByMachineDeploymentName {
		checks = append(checks,
			func(ctx context.Context) preflight.CheckResult {
				return n.vmImageCheckForMachineDetails(
					&nodeSpec.MachineDetails,
					fmt.Sprintf(
						"cluster.spec.topology.workers.machineDeployments[.name=%s]"+
							".overrides[.name=workerConfig].value.nutanix.machineDetails",
						mdName,
					),
				)
			},
		)
	}
	return checks
}

func errorCheck(msg, field string) preflight.Check {
	return func(ctx context.Context) preflight.CheckResult {
		return preflight.CheckResult{
			Name:    "Nutanix",
			Allowed: false,
			Error:   true,
			Causes: []preflight.Cause{
				{
					Message: msg,
					Field:   field,
				},
			},
		}
	}
}

func getData(
	cluster *clusterv1.Cluster,
) (*carenv1.NutanixSpec, *carenv1.NutanixNodeSpec, map[string]*carenv1.NutanixNodeSpec, error) {
	nutanixSpec := &carenv1.NutanixSpec{}
	controlPlaneNutanixNodeSpec := &carenv1.NutanixNodeSpec{}
	nutanixNodeSpecByMachineDeploymentName := make(map[string]*carenv1.NutanixNodeSpec)

	clusterConfig, err := variables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to unmarshal .variables[.name=clusterConfig]: %w", err)
	}
	if clusterConfig != nil && clusterConfig.Nutanix != nil {
		nutanixSpec = clusterConfig.Nutanix
	}

	if clusterConfig.ControlPlane != nil && clusterConfig.ControlPlane.Nutanix != nil {
		controlPlaneNutanixNodeSpec = clusterConfig.ControlPlane.Nutanix
	}

	if cluster.Spec.Topology.Workers != nil {
		for _, md := range cluster.Spec.Topology.Workers.MachineDeployments {
			workerConfig, err := variables.UnmarshalWorkerConfigVariable(md.Variables.Overrides)
			if err != nil {
				return nil, nil, nil, fmt.Errorf(
					"failed to unmarshal .variables[.name=workerConfig] for machine deployment %s: %w",
					md.Name, err,
				)
			}
			if workerConfig != nil && workerConfig.Nutanix != nil {
				nutanixNodeSpecByMachineDeploymentName[md.Name] = workerConfig.Nutanix
			}
		}
	}

	return nutanixSpec, controlPlaneNutanixNodeSpec, nutanixNodeSpecByMachineDeploymentName, nil
}
