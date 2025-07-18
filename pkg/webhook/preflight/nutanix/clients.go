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
	GetCurrentLoggedInUser(ctx context.Context) (*prismv3.UserIntentResponse, error)

	GetImageById(id *string) (*vmmv4.GetImageApiResponse, error)
	ListImages(page_ *int,
		limit_ *int,
		filter_ *string,
		orderby_ *string,
		select_ *string,
		args ...map[string]interface{},
	) (
		*vmmv4.ListImagesApiResponse,
		error,
	)
	GetClusterById(id *string) (*clustermgmtv4.GetClusterApiResponse, error)
	ListClusters(
		page_ *int,
		limit_ *int,
		filter_ *string,
		orderby_ *string,
		apply_ *string,
		select_ *string,
		args ...map[string]interface{},
	) (*clustermgmtv4.ListClustersApiResponse, error)
	ListStorageContainers(
		page_ *int,
		limit_ *int,
		filter_ *string,
		orderby_ *string,
		select_ *string,
		args ...map[string]interface{},
	) (*clustermgmtv4.ListStorageContainersApiResponse, error)
	GetSubnetById(id *string) (*netv4.GetSubnetApiResponse, error)
	ListSubnets(
		page_ *int,
		limit_ *int,
		filter_ *string,
		orderby_ *string,
		expand_ *string,
		select_ *string,
		args ...map[string]interface{},
	) (*netv4.ListSubnetsApiResponse, error)
}

// clientWrapper implements the client interface and wraps both v3 and v4 clients.
type clientWrapper struct {
	v3client *prismv3.Client
	v4client *prismv4.Client
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
		v3client: v3c,
		v4client: v4c,
	}, nil
}

func (c *clientWrapper) GetCurrentLoggedInUser(ctx context.Context) (*prismv3.UserIntentResponse, error) {
	return c.v3client.V3.GetCurrentLoggedInUser(ctx)
}

func (c *clientWrapper) GetImageById(id *string) (*vmmv4.GetImageApiResponse, error) {
	resp, err := c.v4client.ImagesApiInstance.GetImageById(id)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *clientWrapper) ListImages(page_ *int,
	limit_ *int,
	filter_ *string,
	orderby_ *string,
	select_ *string,
	args ...map[string]interface{},
) (*vmmv4.ListImagesApiResponse, error) {
	resp, err := c.v4client.ImagesApiInstance.ListImages(
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

func (c *clientWrapper) GetClusterById(id *string) (*clustermgmtv4.GetClusterApiResponse, error) {
	resp, err := c.v4client.ClustersApiInstance.GetClusterById(id)
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
) (*clustermgmtv4.ListClustersApiResponse, error) {
	resp, err := c.v4client.ClustersApiInstance.ListClusters(
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
) (*clustermgmtv4.ListStorageContainersApiResponse, error) {
	resp, err := c.v4client.StorageContainerAPI.ListStorageContainers(
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

func (c *clientWrapper) GetSubnetById(id *string) (*netv4.GetSubnetApiResponse, error) {
	resp, err := c.v4client.SubnetsApiInstance.GetSubnetById(id)
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
) (*netv4.ListSubnetsApiResponse, error) {
	resp, err := c.v4client.SubnetsApiInstance.ListSubnets(
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
