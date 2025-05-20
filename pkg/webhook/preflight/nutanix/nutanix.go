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
	client        ctrlclient.Client
	nutanixClient *prismv4.Client
	cluster       *clusterv1.Cluster
	clusterConfig *variables.ClusterConfigSpec
}

func (n *Checker) Provider() string {
	return "nutanix"
}

func (n *Checker) Checks(
	ctx context.Context,
	client ctrlclient.Client,
	cluster *clusterv1.Cluster,
) ([]preflight.Check, error) {
	n.client = client

	clusterConfig, err := variables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal topology variable %q: %w", carenv1.ClusterConfigVariableName, err)
	}

	if clusterConfig.Nutanix == nil {
		return nil, fmt.Errorf("missing Nutanix configuration in cluster topology")
	}

	// Initialize Nutanix client from the credentials referenced by the cluster configuration.
	n.nutanixClient, err = newV4Client(ctx, client, cluster.Namespace, clusterConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Nutanix client: %w", err)
	}

	checks := []preflight.Check{}
	if clusterConfig.ControlPlane != nil && clusterConfig.ControlPlane.Nutanix != nil {
		checks = append(
			checks,
			n.VMImageCheck(clusterConfig.ControlPlane.Nutanix.MachineDetails, "controlPlane.nutanix.machineDetails"),
		)
	}

	if cluster.Spec.Topology.Workers != nil {
		for i, md := range cluster.Spec.Topology.Workers.MachineDeployments {
			if md.Variables == nil {
				continue
			}

			workerConfig, err := variables.UnmarshalWorkerConfigVariable(md.Variables.Overrides)
			if err != nil {
				return nil, fmt.Errorf(
					"failed to unmarshal topology variable %q %d: %w",
					carenv1.WorkerConfigVariableName,
					i,
					err,
				)
			}

			if workerConfig.Nutanix == nil {
				continue
			}

			n.VMImageCheck(
				workerConfig.Nutanix.MachineDetails,
				fmt.Sprintf(
					"workers.machineDeployments[.name=%s].variables.overrides[.name=workerConfig].value.nutanix.machineDetails",
					md.Name,
				),
			)
		}
	}

	return checks, nil
}
