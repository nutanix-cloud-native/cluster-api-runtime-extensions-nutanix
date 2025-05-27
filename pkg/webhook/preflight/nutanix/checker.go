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
	preflightutil "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight/util"
)

type Checker struct {
	client  ctrlclient.Client
	cluster *clusterv1.Cluster

	nutanixClient   *prismv4.Client
	variablesGetter *preflightutil.VariablesGetter
}

func (n *Checker) Init(
	ctx context.Context,
	client ctrlclient.Client,
	cluster *clusterv1.Cluster,
) []preflight.Check {
	n.client = client
	n.cluster = cluster
	n.variablesGetter = preflightutil.NewVariablesGetter(cluster)

	// Initialize the Nutanix client. If it fails, return a check that indicates the error.
	clusterConfig, err := n.variablesGetter.ClusterConfig()
	if err != nil {
		return []preflight.Check{
			func(ctx context.Context) preflight.CheckResult {
				return preflight.CheckResult{
					Name:    "NutanixClientInitialization",
					Allowed: false,
					Error:   true,
					Causes: []preflight.Cause{
						{
							Message: fmt.Sprintf("failed to read clusterConfig variable: %s", err),
							Field:   "cluster.spec.topology.variables",
						},
					},
				}
			},
		}
	}

	n.nutanixClient, err = v4client(ctx, client, cluster, clusterConfig.Nutanix)
	// TODO Verify the credentials by making a users API call.
	if err != nil {
		return []preflight.Check{
			func(ctx context.Context) preflight.CheckResult {
				return preflight.CheckResult{
					Name:    "NutanixClientInitialization",
					Allowed: false,
					Error:   true,
					Causes: []preflight.Cause{
						{
							Message: fmt.Sprintf("failed to initialize Nutanix client: %s", err),
							Field:   "cluster.spec.topology.variables[.name=clusterConfig].nutanix",
						},
					},
				}
			},
		}
	}

	return []preflight.Check{
		n.VMImages,
	}
}
