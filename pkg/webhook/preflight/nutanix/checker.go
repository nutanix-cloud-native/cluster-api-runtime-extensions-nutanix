// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"

	prismv4 "github.com/nutanix-cloud-native/prism-go-client/v4"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

type Checker struct{}

func (n *Checker) Init(
	ctx context.Context,
	kclient ctrlclient.Client,
	cluster *clusterv1.Cluster,
) []preflight.Check {
	// Read Nutanix specs from the cluster.
	prismCentralEndpointSpec,
		controlPlaneNutanixNodeSpec,
		nutanixNodeSpecByMachineDeploymentName,
		errCauses := specsFromCluster(cluster)
	if len(errCauses) > 0 {
		return initErrorCheck(errCauses...)
	}

	// If no Nutanix specs are found, no checks need to run.
	if controlPlaneNutanixNodeSpec == nil && len(nutanixNodeSpecByMachineDeploymentName) == 0 {
		return nil
	}

	// Create a Nutanix client, because all checks require it.
	nv4client, errCauses := newV4Client(ctx, kclient, cluster.Namespace, prismCentralEndpointSpec)
	if len(errCauses) > 0 {
		return initErrorCheck(errCauses...)
	}

	// Initialize checks.
	checks := []preflight.Check{}
	if controlPlaneNutanixNodeSpec != nil {
		checks = append(checks,
			newVMImageCheck(
				nv4client,
				controlPlaneNutanixNodeSpec,
				"cluster.spec.topology[.name=clusterConfig].value.controlPlane.nutanix",
			),
		)
	}
	for _, md := range cluster.Spec.Topology.Workers.MachineDeployments {
		if nutanixNodeSpecByMachineDeploymentName[md.Name] == nil {
			continue
		}
		checks = append(checks,
			newVMImageCheck(
				nv4client,
				nutanixNodeSpecByMachineDeploymentName[md.Name],
				fmt.Sprintf(
					"cluster.spec.topology.workers.machineDeployments[.name=%s].variables[.name=workerConfig].value.nutanix",
					md.Name,
				),
			),
		)
	}
	return checks
}

func initErrorCheck(causes ...preflight.Cause) []preflight.Check {
	return []preflight.Check{
		func(ctx context.Context) preflight.CheckResult {
			return preflight.CheckResult{
				Name:    "Nutanix",
				Allowed: false,
				Error:   true,
				Causes:  causes,
			}
		},
	}
}
