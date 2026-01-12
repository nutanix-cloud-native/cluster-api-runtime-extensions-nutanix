//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	networkingcommonapi "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/common/v1/config"
	networkingapi "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"
	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/prism-go-client/converged"
	v4Converged "github.com/nutanix-cloud-native/prism-go-client/converged/v4"
)

func GetSubnetUUIDFromNameAndCluster(
	ctx context.Context,
	subnet, cluster string,
	convergedClient *v4Converged.Client,
) (uuid.UUID, error) {
	clusterUUID, err := uuid.Parse(cluster)
	if err != nil {
		clusterUUID, err = GetClusterUUIDFromName(ctx, cluster, convergedClient)
		if err != nil {
			return uuid.UUID{}, fmt.Errorf(
				"failed to get cluster uuid for cluster %s: %w",
				cluster,
				err,
			)
		}
	}

	subnetList, err := convergedClient.Subnets.List(
		ctx,
		converged.WithFilter(`name eq '`+subnet+`'  and clusterReference eq '`+clusterUUID.String()+`'`),
	)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to find subnet uuid for subnet %s: %w", subnet, err)
	}

	if len(subnetList) == 0 {
		return uuid.UUID{}, fmt.Errorf("no subnet found with name %s", subnet)
	}

	subnetUUID, err := uuid.Parse(*subnetList[0].ExtId)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to parse subnet uuid for subnet %s: %w", subnet, err)
	}

	return subnetUUID, nil
}

type reservedIPs struct {
	ReservedIPs []string `json:"reserved_ips"`
}

func ReserveIP(
	ctx context.Context,
	subnet, cluster string,
	convergedClient *v4Converged.Client,
) (ip string, unreserve func() error, err error) {
	clusterUUID, err := uuid.Parse(cluster)
	if err != nil {
		clusterUUID, err = GetClusterUUIDFromName(ctx, cluster, convergedClient)
		if err != nil {
			return "", nil, fmt.Errorf(
				"failed to get cluster uuid for cluster %s: %w",
				cluster,
				err,
			)
		}
	}

	subnetUUID, err := uuid.Parse(subnet)
	if err != nil {
		subnetUUID, err = GetSubnetUUIDFromNameAndCluster(ctx, subnet, clusterUUID.String(), convergedClient)
		if err != nil {
			return "", nil, fmt.Errorf("failed to get subnet uuid for subnet %s: %w", subnet, err)
		}
	}

	taskRef, err := convergedClient.Subnets.ReserveIpsBySubnetId(
		ctx,
		subnetUUID.String(),
		&networkingapi.IpReserveSpec{
			Count:       ptr.To[int64](1),
			ReserveType: ptr.To(networkingapi.RESERVETYPE_IP_ADDRESS_COUNT),
		},
	)
	if err != nil {
		return "", nil, fmt.Errorf("failed to reserve IP in subnet %s: %w", subnet, err)
	}

	// The converged client returns TaskReference directly
	if taskRef == nil || taskRef.ExtId == nil {
		return "", nil, fmt.Errorf("no task id found in response: %+v", taskRef)
	}
	taskRefData := taskRef

	taskCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := WaitForTaskCompletion(taskCtx, *taskRefData.ExtId, convergedClient)
	if err != nil {
		return "", nil, fmt.Errorf("failed to wait for task completion: %w", err)
	}

	if len(result) == 0 {
		return "", nil, fmt.Errorf("no IP address reserved")
	}

	marshaledResponseBytes, _ := json.Marshal(result[0].Value)
	marshaledResponse, err := strconv.Unquote(string(marshaledResponseBytes))
	if err != nil {
		return "", nil, fmt.Errorf(
			"failed to unquote reserved IP response %s: %w",
			marshaledResponseBytes,
			err,
		)
	}

	var response reservedIPs
	if err := json.Unmarshal([]byte(marshaledResponse), &response); err != nil {
		return "", nil, fmt.Errorf(
			"failed to unmarshal reserved IP response %s: %w",
			marshaledResponse,
			err,
		)
	}

	return response.ReservedIPs[0],
		func() error {
			return UnreserveIP(
				ctx,
				response.ReservedIPs[0],
				subnetUUID.String(),
				clusterUUID.String(),
				convergedClient,
			)
		},
		nil
}

func UnreserveIP(
	ctx context.Context,
	ip, subnet, cluster string,
	convergedClient *v4Converged.Client,
) error {
	clusterUUID, err := uuid.Parse(cluster)
	if err != nil {
		clusterUUID, err = GetClusterUUIDFromName(ctx, cluster, convergedClient)
		if err != nil {
			return fmt.Errorf("failed to get cluster uuid for cluster %s: %w", cluster, err)
		}
	}

	subnetUUID, err := uuid.Parse(subnet)
	if err != nil {
		subnetUUID, err = GetSubnetUUIDFromNameAndCluster(ctx, subnet, clusterUUID.String(), convergedClient)
		if err != nil {
			return fmt.Errorf("failed to get subnet uuid for subnet %s: %w", subnet, err)
		}
	}

	ipAddress := networkingcommonapi.NewIPAddress()
	ipAddress.Ipv4 = networkingcommonapi.NewIPv4Address()
	ipAddress.Ipv4.Value = ptr.To(ip)
	taskRef, err := convergedClient.Subnets.UnreserveIpsBySubnetId(
		ctx,
		subnetUUID.String(),
		&networkingapi.IpUnreserveSpec{
			UnreserveType: ptr.To(networkingapi.UNRESERVETYPE_IP_ADDRESS_LIST),
			IpAddresses:   []networkingcommonapi.IPAddress{*ipAddress},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to unreserve IP in subnet %s: %w", subnet, err)
	}

	// The converged client returns TaskReference directly
	if taskRef == nil || taskRef.ExtId == nil {
		return fmt.Errorf("no task id found in response: %+v", taskRef)
	}
	taskRefData := taskRef

	taskCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err = WaitForTaskCompletion(taskCtx, *taskRefData.ExtId, convergedClient)
	if err != nil {
		return fmt.Errorf("failed to wait for task completion: %w", err)
	}

	return nil
}
