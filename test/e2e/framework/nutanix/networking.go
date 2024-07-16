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
	networkingprismapi "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/prism/v4/config"
	"k8s.io/utils/ptr"

	prismclientv4 "github.com/nutanix-cloud-native/prism-go-client/v4"
)

func GetSubnetUUIDFromNameAndCluster(
	subnet, cluster string, v4Client *prismclientv4.Client,
) (uuid.UUID, error) {
	clusterUUID, err := uuid.Parse(cluster)
	if err != nil {
		clusterUUID, err = GetClusterUUIDFromName(cluster, v4Client)
		if err != nil {
			return uuid.UUID{}, fmt.Errorf(
				"failed to get cluster uuid for cluster %s: %w",
				cluster,
				err,
			)
		}
	}

	listSubnetsResponse, err := v4Client.SubnetsApiInstance.ListSubnets(
		nil,
		nil,
		ptr.To(`name eq '`+subnet+`'  and clusterReference eq '`+clusterUUID.String()+`'`),
		nil,
		nil,
		nil,
	)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to find subnet uuid for subnet %s: %w", subnet, err)
	}
	subnets := listSubnetsResponse.GetData()
	if subnets == nil {
		return uuid.UUID{}, fmt.Errorf("no subnet found with name %s", subnet)
	}

	switch apiSubnets := subnets.(type) {
	case []networkingapi.Subnet:
		if len(apiSubnets) == 0 {
			return uuid.UUID{}, fmt.Errorf("no subnet found with name %s", subnet)
		}

		subnetUUID, err := uuid.Parse(*apiSubnets[0].ExtId)
		if err != nil {
			return uuid.UUID{}, fmt.Errorf("failed to parse subnet uuid for subnet %s: %w", subnet, err)
		}

		return subnetUUID, nil
	case []networkingapi.SubnetProjection:
		if len(apiSubnets) == 0 {
			return uuid.UUID{}, fmt.Errorf("no subnet found with name %s", subnet)
		}

		subnetUUID, err := uuid.Parse(*apiSubnets[0].ExtId)
		if err != nil {
			return uuid.UUID{}, fmt.Errorf("failed to parse subnet uuid for subnet %s: %w", subnet, err)
		}

		return subnetUUID, nil
	default:
		return uuid.UUID{}, fmt.Errorf("unknown response: %+v", subnets)
	}
}

type reservedIPs struct {
	ReservedIPs []string `json:"reserved_ips"`
}

func ReserveIP(
	subnet, cluster string,
	v4Client *prismclientv4.Client,
) (ip string, unreserve func() error, err error) {
	clusterUUID, err := uuid.Parse(cluster)
	if err != nil {
		clusterUUID, err = GetClusterUUIDFromName(cluster, v4Client)
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
		subnetUUID, err = GetSubnetUUIDFromNameAndCluster(subnet, clusterUUID.String(), v4Client)
		if err != nil {
			return "", nil, fmt.Errorf("failed to get subnet uuid for subnet %s: %w", subnet, err)
		}
	}

	reserveIPResponse, err := v4Client.SubnetIPReservationApi.ReserveIpsBySubnetId(
		ptr.To(subnetUUID.String()),
		&networkingapi.IpReserveSpec{
			Count:       ptr.To[int64](1),
			ReserveType: ptr.To(networkingapi.RESERVETYPE_IP_ADDRESS_COUNT),
		},
	)
	if err != nil {
		return "", nil, fmt.Errorf("failed to reserve IP in subnet %s: %w", subnet, err)
	}

	responseData, ok := reserveIPResponse.GetData().(networkingprismapi.TaskReference)
	if !ok {
		return "", nil, fmt.Errorf(
			"unexpected response data type %[1]T: %+[1]v",
			reserveIPResponse.GetData(),
		)
	}
	if responseData.ExtId == nil {
		return "", nil, fmt.Errorf(
			"no task id found in response: %+[1]v",
			reserveIPResponse.GetData(),
		)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := WaitForTaskCompletion(ctx, *responseData.ExtId, v4Client)
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
				response.ReservedIPs[0],
				subnetUUID.String(),
				clusterUUID.String(),
				v4Client,
			)
		},
		nil
}

func UnreserveIP(ip, subnet, cluster string, v4Client *prismclientv4.Client) error {
	clusterUUID, err := uuid.Parse(cluster)
	if err != nil {
		clusterUUID, err = GetClusterUUIDFromName(cluster, v4Client)
		if err != nil {
			return fmt.Errorf("failed to get cluster uuid for cluster %s: %w", cluster, err)
		}
	}

	subnetUUID, err := uuid.Parse(subnet)
	if err != nil {
		subnetUUID, err = GetSubnetUUIDFromNameAndCluster(subnet, clusterUUID.String(), v4Client)
		if err != nil {
			return fmt.Errorf("failed to get subnet uuid for subnet %s: %w", subnet, err)
		}
	}

	ipAddress := networkingcommonapi.NewIPAddress()
	ipAddress.Ipv4 = networkingcommonapi.NewIPv4Address()
	ipAddress.Ipv4.Value = ptr.To(ip)
	unreserveIPResponse, err := v4Client.SubnetIPReservationApi.UnreserveIpsBySubnetId(
		ptr.To(subnetUUID.String()),
		&networkingapi.IpUnreserveSpec{
			UnreserveType: ptr.To(networkingapi.UNRESERVETYPE_IP_ADDRESS_LIST),
			IpAddresses:   []networkingcommonapi.IPAddress{*ipAddress},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to reserve IP in subnet %s: %w", subnet, err)
	}

	responseData, ok := unreserveIPResponse.GetData().(networkingprismapi.TaskReference)
	if !ok {
		return fmt.Errorf(
			"unexpected response data type %[1]T: %+[1]v",
			unreserveIPResponse.GetData(),
		)
	}
	if responseData.ExtId == nil {
		return fmt.Errorf("no task id found in response: %+v", unreserveIPResponse.GetData())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = WaitForTaskCompletion(ctx, *responseData.ExtId, v4Client)
	if err != nil {
		return fmt.Errorf("failed to wait for task completion: %w", err)
	}

	return nil
}
