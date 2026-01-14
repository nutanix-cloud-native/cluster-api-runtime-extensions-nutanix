//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/nutanix-cloud-native/prism-go-client/converged"
	v4Converged "github.com/nutanix-cloud-native/prism-go-client/converged/v4"
)

func GetClusterUUIDFromName(
	ctx context.Context,
	cluster string,
	convergedClient *v4Converged.Client,
) (uuid.UUID, error) {
	clusterUUID, err := uuid.Parse(cluster)
	if err == nil {
		return clusterUUID, nil
	}

	clusterList, err := convergedClient.Clusters.List(
		ctx,
		converged.WithFilter(`name eq '`+cluster+`'`),
	)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf(
			"failed to find cluster uuid for cluster %s: %w",
			cluster,
			err,
		)
	}

	if len(clusterList) == 0 {
		return uuid.UUID{}, fmt.Errorf("no cluster found with name %s", cluster)
	}

	clusterUUID, err = uuid.Parse(*clusterList[0].ExtId)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to parse cluster uuid for cluster %s: %w", cluster, err)
	}

	return clusterUUID, nil
}
