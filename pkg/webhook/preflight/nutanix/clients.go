// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"

	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	netv4 "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"
	vmmv4 "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/content"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	prismv3 "github.com/nutanix-cloud-native/prism-go-client/v3"
	prismv4 "github.com/nutanix-cloud-native/prism-go-client/v4"
)

// client contains methods to interact with Nutanix Prism v3 and v4 APIs.
type client interface {
	GetCurrentLoggedInUser(
		ctx context.Context,
	) (
		*prismv3.UserIntentResponse,
		error,
	)

	GetImageById(
		uuid *string,
		args ...map[string]interface{},
	) (
		*vmmv4.GetImageApiResponse,
		error,
	)

	ListImages(
		page_ *int,
		limit_ *int,
		filter_ *string,
		orderby_ *string,
		select_ *string,
		args ...map[string]interface{},
	) (
		*vmmv4.ListImagesApiResponse,
		error,
	)

	GetClusterById(
		uuid *string,
		args ...map[string]interface{},
	) (
		*clustermgmtv4.GetClusterApiResponse, error,
	)

	ListClusters(
		page_ *int,
		limit_ *int,
		filter_ *string,
		orderby_ *string,
		apply_ *string,
		select_ *string,
		args ...map[string]interface{},
	) (
		*clustermgmtv4.ListClustersApiResponse,
		error,
	)
	ListStorageContainers(
		page_ *int,
		limit_ *int,
		filter_ *string,
		orderby_ *string,
		select_ *string,
		args ...map[string]interface{},
	) (
		*clustermgmtv4.ListStorageContainersApiResponse,
		error,
	)

	GetSubnetById(
		uuid *string,
		args ...map[string]interface{},
	) (
		*netv4.GetSubnetApiResponse, error,
	)

	ListSubnets(
		page_ *int,
		limit_ *int,
		filter_ *string,
		orderby_ *string,
		expand_ *string,
		select_ *string,
		args ...map[string]interface{},
	) (
		*netv4.ListSubnetsApiResponse, error,
	)
}

// clientWrapper implements the client interface and wraps both v3 and v4 clients.
type clientWrapper struct {
	GetCurrentLoggedInUserFunc func(
		ctx context.Context,
	) (
		*prismv3.UserIntentResponse, error,
	)

	GetImageByIdFunc func(
		uuid *string,
		args ...map[string]interface{},
	) (
		*vmmv4.GetImageApiResponse, error,
	)

	ListImagesFunc func(
		page_ *int,
		limit_ *int,
		filter_ *string,
		orderby_ *string,
		select_ *string,
		args ...map[string]interface{},
	) (
		*vmmv4.ListImagesApiResponse,
		error,
	)

	GetClusterByIdFunc func(
		uuid *string,
		args ...map[string]interface{},
	) (
		*clustermgmtv4.GetClusterApiResponse, error,
	)

	ListClustersFunc func(
		page_ *int,
		limit_ *int,
		filter_ *string,
		orderby_ *string,
		apply_ *string,
		select_ *string,
		args ...map[string]interface{},
	) (
		*clustermgmtv4.ListClustersApiResponse,
		error,
	)
	ListStorageContainersFunc func(
		page_ *int,
		limit_ *int,
		filter_ *string,
		orderby_ *string,
		select_ *string,
		args ...map[string]interface{},
	) (
		*clustermgmtv4.ListStorageContainersApiResponse,
		error,
	)

	GetSubnetByIdFunc func(
		uuid *string,
		args ...map[string]interface{},
	) (
		*netv4.GetSubnetApiResponse, error,
	)

	ListSubnetsFunc func(
		page_ *int,
		limit_ *int,
		filter_ *string,
		orderby_ *string,
		expand_ *string,
		select_ *string,
		args ...map[string]interface{},
	) (
		*netv4.ListSubnetsApiResponse, error,
	)
}

var _ = client(&clientWrapper{})

func newClient(
	credentials prismgoclient.Credentials, //nolint:gocritic // hugeParam is fine
) (client, error) {
	v3c, err := prismv3.NewV3Client(credentials)
	if err != nil {
		return nil, fmt.Errorf("failed to create v3 client: %w", err)
	}

	v4c, err := prismv4.NewV4Client(credentials)
	if err != nil {
		return nil, fmt.Errorf("failed to create v4 client: %w", err)
	}

	return &clientWrapper{
		GetCurrentLoggedInUserFunc: v3c.V3.GetCurrentLoggedInUser,
		GetImageByIdFunc:           v4c.ImagesApiInstance.GetImageById,
		ListImagesFunc:             v4c.ImagesApiInstance.ListImages,
		GetClusterByIdFunc:         v4c.ClustersApiInstance.GetClusterById,
		ListClustersFunc:           v4c.ClustersApiInstance.ListClusters,
		ListStorageContainersFunc:  v4c.StorageContainerAPI.ListStorageContainers,
		GetSubnetByIdFunc:          v4c.SubnetsApiInstance.GetSubnetById,
		ListSubnetsFunc:            v4c.SubnetsApiInstance.ListSubnets,
	}, nil
}

func (c *clientWrapper) GetCurrentLoggedInUser(
	ctx context.Context,
) (
	*prismv3.UserIntentResponse,
	error,
) {
	return c.GetCurrentLoggedInUserFunc(ctx)
}

func (c *clientWrapper) GetImageById(
	uuid *string,
	args ...map[string]interface{},
) (
	*vmmv4.GetImageApiResponse,
	error,
) {
	resp, err := c.GetImageByIdFunc(
		uuid,
		args...,
	)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *clientWrapper) ListImages(
	page_ *int,
	limit_ *int,
	filter_ *string,
	orderby_ *string,
	select_ *string,
	args ...map[string]interface{},
) (
	*vmmv4.ListImagesApiResponse,
	error,
) {
	resp, err := c.ListImagesFunc(
		page_,
		limit_,
		filter_,
		orderby_,
		select_,
		args...,
	)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *clientWrapper) GetClusterById(
	uuid *string,
	args ...map[string]interface{},
) (
	*clustermgmtv4.GetClusterApiResponse,
	error,
) {
	resp, err := c.GetClusterByIdFunc(
		uuid,
		args...,
	)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *clientWrapper) ListClusters(
	page_ *int,
	limit_ *int,
	filter_ *string,
	orderby_ *string,
	apply_ *string,
	select_ *string,
	args ...map[string]interface{},
) (
	*clustermgmtv4.ListClustersApiResponse,
	error,
) {
	resp, err := c.ListClustersFunc(
		page_,
		limit_,
		filter_,
		orderby_,
		apply_,
		select_,
		args...,
	)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *clientWrapper) ListStorageContainers(
	page_ *int,
	limit_ *int,
	filter_ *string,
	orderby_ *string,
	select_ *string,
	args ...map[string]interface{},
) (
	*clustermgmtv4.ListStorageContainersApiResponse,
	error,
) {
	resp, err := c.ListStorageContainersFunc(
		page_,
		limit_,
		filter_,
		orderby_,
		select_,
		args...,
	)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *clientWrapper) GetSubnetById(
	uuid *string,
	args ...map[string]interface{},
) (
	*netv4.GetSubnetApiResponse,
	error,
) {
	resp, err := c.GetSubnetByIdFunc(
		uuid,
		args...,
	)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *clientWrapper) ListSubnets(
	page_ *int,
	limit_ *int,
	filter_ *string,
	orderby_ *string,
	expand_ *string,
	select_ *string,
	args ...map[string]interface{},
) (
	*netv4.ListSubnetsApiResponse,
	error,
) {
	resp, err := c.ListSubnetsFunc(
		page_,
		limit_,
		filter_,
		orderby_,
		expand_,
		select_,
		args...,
	)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
