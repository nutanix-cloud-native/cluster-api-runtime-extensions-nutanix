// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"sync"

	prismv4 "github.com/nutanix-cloud-native/prism-go-client/v4"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

type Checker struct {
	client        ctrlclient.Client
	nutanixClient *prismv4.Client
	cluster       *clusterv1.Cluster

	clientMutex sync.Mutex
}

func (n *Checker) Init(
	ctx context.Context,
	client ctrlclient.Client,
	cluster *clusterv1.Cluster,
) []preflight.Check {
	n.client = client
	n.cluster = cluster

	checks := []preflight.Check{
		n.VMImageCheck,
	}

	return checks
}
