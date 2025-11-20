//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"fmt"

	"github.com/google/uuid"
	clustersapi "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	"k8s.io/utils/ptr"

	prismclientv4 "github.com/nutanix-cloud-native/prism-go-client/v4"
)

func GetClusterUUIDFromName(cluster string, v4Client *prismclientv4.Client) (uuid.UUID, error) {
	clusterUUID, err := uuid.Parse(cluster)
	if err == nil {
		return clusterUUID, nil
	}

	response, err := v4Client.ClustersApiInstance.ListClusters(
		nil,
		nil,
		ptr.To(`name eq '`+cluster+`'`),
		nil,
		nil,
		nil,
		nil,
	)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf(
			"failed to find cluster uuid for cluster %s: %w",
			cluster,
			err,
		)
	}
	clusters := response.GetData()
	if clusters == nil {
		return uuid.UUID{}, fmt.Errorf("no cluster found with name %s", cluster)
	}

	switch apiClusters := clusters.(type) {
	case []clustersapi.Cluster:
		if len(apiClusters) == 0 {
			return uuid.UUID{}, fmt.Errorf("no subnet found with name %s", cluster)
		}

		clusterUUID, err := uuid.Parse(*apiClusters[0].ExtId)
		if err != nil {
			return uuid.UUID{}, fmt.Errorf("failed to parse cluster uuid for cluster %s: %w", cluster, err)
		}

		return clusterUUID, nil
	default:
		return uuid.UUID{}, fmt.Errorf("unknown response: %+v", clusters)
	}
}
