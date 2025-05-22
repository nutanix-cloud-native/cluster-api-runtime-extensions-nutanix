// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
	preflightutil "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight/util"
)

type Checker struct {
	client  ctrlclient.Client
	cluster *clusterv1.Cluster

	clientGetter    *ClientGetter
	variablesGetter *preflightutil.VariablesGetter
}

func (n *Checker) Init(
	ctx context.Context,
	client ctrlclient.Client,
	cluster *clusterv1.Cluster,
) []preflight.Check {
	n.client = client
	n.cluster = cluster
	n.clientGetter = &ClientGetter{client: client, cluster: cluster}
	n.variablesGetter = preflightutil.NewVariablesGetter(cluster)

	return []preflight.Check{
		n.VMImageCheck,
	}
}
