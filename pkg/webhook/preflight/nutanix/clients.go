// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"

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
