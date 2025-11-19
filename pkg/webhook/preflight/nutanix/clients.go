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
		ctx context.Context,
		uuid *string,
		args ...map[string]interface{},
	) (
		*vmmv4.GetImageApiResponse,
		error,
	)

	ListImages(
		ctx context.Context,
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
		ctx context.Context,
		uuid *string,
		args ...map[string]interface{},
	) (
		*clustermgmtv4.GetClusterApiResponse, error,
	)

	ListClusters(
		ctx context.Context,
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
		ctx context.Context,
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
		ctx context.Context,
		uuid *string,
		args ...map[string]interface{},
	) (
		*netv4.GetSubnetApiResponse, error,
	)

	ListSubnets(
		ctx context.Context,
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
		GetClusterByIdFunc: func(uuid *string, args ...map[string]interface{}) (*clustermgmtv4.GetClusterApiResponse, error) {
			return v4c.ClustersApiInstance.GetClusterById(uuid, nil, args...)
		},
		ListClustersFunc: func(
			page_, limit_ *int,
			filter_, orderby_, apply_, select_ *string,
			args ...map[string]interface{},
		) (*clustermgmtv4.ListClustersApiResponse, error) {
			return v4c.ClustersApiInstance.ListClusters(
				page_, limit_, filter_, orderby_, apply_, nil, select_, args...,
			)
		},
		ListStorageContainersFunc: v4c.StorageContainerAPI.ListStorageContainers,
		GetSubnetByIdFunc:         v4c.SubnetsApiInstance.GetSubnetById,
		ListSubnetsFunc:           v4c.SubnetsApiInstance.ListSubnets,
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
	ctx context.Context,
	uuid *string,
	args ...map[string]interface{},
) (
	*vmmv4.GetImageApiResponse,
	error,
) {
	return callWithContext(ctx, func() (*vmmv4.GetImageApiResponse, error) {
		return c.GetImageByIdFunc(
			uuid,
			args...,
		)
	})
}

func (c *clientWrapper) ListImages(
	ctx context.Context,
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
	return callWithContext(ctx, func() (*vmmv4.ListImagesApiResponse, error) {
		return c.ListImagesFunc(
			page_,
			limit_,
			filter_,
			orderby_,
			select_,
			args...,
		)
	})
}

func (c *clientWrapper) GetClusterById(
	ctx context.Context,
	uuid *string,
	args ...map[string]interface{},
) (
	*clustermgmtv4.GetClusterApiResponse,
	error,
) {
	return callWithContext(ctx, func() (*clustermgmtv4.GetClusterApiResponse, error) {
		return c.GetClusterByIdFunc(
			uuid,
			args...,
		)
	})
}

func (c *clientWrapper) ListClusters(
	ctx context.Context,
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
	return callWithContext(ctx, func() (*clustermgmtv4.ListClustersApiResponse, error) {
		return c.ListClustersFunc(
			page_,
			limit_,
			filter_,
			orderby_,
			apply_,
			select_,
			args...,
		)
	})
}

func (c *clientWrapper) ListStorageContainers(
	ctx context.Context,
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
	return callWithContext(ctx, func() (*clustermgmtv4.ListStorageContainersApiResponse, error) {
		return c.ListStorageContainersFunc(
			page_,
			limit_,
			filter_,
			orderby_,
			select_,
			args...,
		)
	})
}

func (c *clientWrapper) GetSubnetById(
	ctx context.Context,
	uuid *string,
	args ...map[string]interface{},
) (
	*netv4.GetSubnetApiResponse,
	error,
) {
	return callWithContext(ctx, func() (*netv4.GetSubnetApiResponse, error) {
		return c.GetSubnetByIdFunc(
			uuid,
			args...,
		)
	})
}

func (c *clientWrapper) ListSubnets(
	ctx context.Context,
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
	return callWithContext(ctx, func() (*netv4.ListSubnetsApiResponse, error) {
		return c.ListSubnetsFunc(
			page_,
			limit_,
			filter_,
			orderby_,
			expand_,
			select_,
			args...,
		)
	})
}

// callWithContext is a helper function that immediately responds to context cancellation,
// while calling a long-running, non-preemptible function. The long-running function always
// runs to completion, but its result is only returned if the context is not cancelled.
func callWithContext[T any](ctx context.Context, f func() (T, error)) (T, error) {
	type result[T any] struct {
		val T
		err error
	}

	// The buffered channel allows us to send the result of the function without knowing
	// whether the channel will be read. If the context is not cancelled, this function will
	// read from the channel and return the value to caller. If the context is cancelled, we
	// will not read from the channel. Once this function returns, the channel will go out
	// of scope, and be garbage collected.
	ch := make(chan result[T], 1)

	go func() {
		resp, err := f()
		ch <- result[T]{val: resp, err: err}
	}()

	select {
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	case res := <-ch:
		return res.val, res.err
	}
}
