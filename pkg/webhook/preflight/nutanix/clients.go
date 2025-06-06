// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"

	vmmv4 "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/content"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	prismv3 "github.com/nutanix-cloud-native/prism-go-client/v3"
	prismv4 "github.com/nutanix-cloud-native/prism-go-client/v4"
)

type v3client interface {
	GetCurrentLoggedInUser(ctx context.Context) (*prismv3.UserIntentResponse, error)
}

type v3clientWrapper struct {
	prismv3.Service
}

var _ = v3client(&v3clientWrapper{})

func newV3Client(
	credentials prismgoclient.Credentials, //nolint:gocritic // hugeParam is fine
) (v3client, error) {
	client, err := prismv3.NewV3Client(credentials)
	if err != nil {
		return nil, err
	}
	return &v3clientWrapper{
		client.V3,
	}, nil
}

type v4client interface {
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

type v4clientWrapper struct {
	client *prismv4.Client
}

var _ = v4client(&v4clientWrapper{})

func (c *v4clientWrapper) GetImageById(id *string) (*vmmv4.GetImageApiResponse, error) {
	resp, err := c.client.ImagesApiInstance.GetImageById(id)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *v4clientWrapper) ListImages(page_ *int,
	limit_ *int,
	filter_ *string,
	orderby_ *string,
	select_ *string,
	args ...map[string]interface{},
) (*vmmv4.ListImagesApiResponse, error) {
	resp, err := c.client.ImagesApiInstance.ListImages(
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

func newV4Client(
	credentials prismgoclient.Credentials, //nolint:gocritic // hugeParam is fine
) (v4client, error) {
	client, err := prismv4.NewV4Client(credentials)
	if err != nil {
		return nil, err
	}
	return &v4clientWrapper{
		client: client,
	}, nil
}
