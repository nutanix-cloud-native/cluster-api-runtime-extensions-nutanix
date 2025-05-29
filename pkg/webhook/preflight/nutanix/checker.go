// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	prismv4 "github.com/nutanix-cloud-native/prism-go-client/v4"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

type Checker struct{}

func (n *Checker) Init(
	ctx context.Context,
	kclient ctrlclient.Client,
	cluster *clusterv1.Cluster,
) []preflight.Check {
	prismCentralEndpointSpec,
		controlPlaneNutanixNodeSpec,
		nutanixNodeSpecByMachineDeploymentName,
		errCauses := readNutanixSpecs(cluster)
	if len(errCauses) > 0 {
		return initErrorCheck(errCauses...)
	}

	if controlPlaneNutanixNodeSpec == nil && len(nutanixNodeSpecByMachineDeploymentName) == 0 {
		// No Nutanix specs found, no checks to run.
		return nil
	}

	// At least one Nutanix spec is defined. Get credentials and create a client,
	// because all checks require them.
	credentials, errCauses := getCredentials(ctx, kclient, cluster, prismCentralEndpointSpec)
	if len(errCauses) > 0 {
		return initErrorCheck(errCauses...)
	}

	nv4client, err := prismv4.NewV4Client(*credentials)
	if err != nil {
		return initErrorCheck(
			preflight.Cause{
				Message: fmt.Sprintf("failed to initialize Nutanix v4 client: %s", err),
				Field:   "",
			})
	}

	// Initialize checks.
	checks := []preflight.Check{}
	if controlPlaneNutanixNodeSpec != nil {
		checks = append(checks,
			vmImageCheck(
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
			vmImageCheck(
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
