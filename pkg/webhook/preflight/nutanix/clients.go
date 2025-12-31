// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	netv4 "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"
	vmmv4 "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/content"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	"github.com/nutanix-cloud-native/prism-go-client/converged"
	"github.com/nutanix-cloud-native/prism-go-client/environment/types"
	prismv3 "github.com/nutanix-cloud-native/prism-go-client/v3"
)

// client contains methods to interact with Nutanix Prism v3 API and converged v4 client.
type client interface {
	GetCurrentLoggedInUser(
		ctx context.Context,
	) (
		*prismv3.UserIntentResponse,
		error,
	)

	GetPrismCentralVersion(
		ctx context.Context,
	) (
		string,
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

// clientWrapper implements the client interface and wraps v3 and converged v4 clients.
type clientWrapper struct {
	GetCurrentLoggedInUserFunc func(
		ctx context.Context,
	) (
		*prismv3.UserIntentResponse, error,
	)

	GetPrismCentralVersionFunc func(
		ctx context.Context,
	) (
		string, error,
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

// ErrEmptyHostInURL is returned when the parsed URL has an empty host.
var ErrEmptyHostInURL = errors.New("parsed URL has empty host")

// buildODataOptions converts pointer-based OData parameters to functional options.
func buildODataOptions(page, limit *int, filter, orderby, selectFields *string) []converged.ODataOption {
	var opts []converged.ODataOption
	if page != nil {
		opts = append(opts, converged.WithPage(*page))
	}
	if limit != nil {
		opts = append(opts, converged.WithLimit(*limit))
	}
	if filter != nil && *filter != "" {
		opts = append(opts, converged.WithFilter(*filter))
	}
	if orderby != nil && *orderby != "" {
		opts = append(opts, converged.WithOrderBy(*orderby))
	}
	if selectFields != nil && *selectFields != "" {
		opts = append(opts, converged.WithSelect(*selectFields))
	}
	return opts
}

// buildManagementEndpoint creates a ManagementEndpoint from credentials and trust bundle.
func buildManagementEndpoint(credentials *prismgoclient.Credentials) (*types.ManagementEndpoint, error) {
	urlStr := credentials.URL

	// Prepend https:// if no scheme is present
	// Nutanix Prism Central URLs may be provided as "host:port" without scheme
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		urlStr = "https://" + urlStr
	}

	// Parse URL - preserve existing scheme if present (e.g., for test servers)
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL %q: %w", urlStr, err)
	}

	// Validate that we have a host after parsing
	if parsedURL.Host == "" {
		return nil, fmt.Errorf("invalid URL %q: %w", credentials.URL, ErrEmptyHostInURL)
	}

	return &types.ManagementEndpoint{
		Address:  parsedURL,
		Insecure: credentials.Insecure,
		ApiCredentials: types.ApiCredentials{
			Username: credentials.Username,
			Password: credentials.Password,
		},
	}, nil
}

func newClient(
	credentials prismgoclient.Credentials, //nolint:gocritic // hugeParam is fine
) (client, error) {
	v3c, err := prismv3.NewV3Client(credentials)
	if err != nil {
		return nil, fmt.Errorf("failed to create v3 client: %w", err)
	}

	endpoint, err := buildManagementEndpoint(&credentials)
	if err != nil {
		return nil, fmt.Errorf("failed to build management endpoint: %w", err)
	}
	cacheParams := &CacheParams{
		PrismManagementEndpoint: endpoint,
	}

	convergedc, err := NutanixConvergedClientV4Cache.GetOrCreate(cacheParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create converged client: %w", err)
	}

	return &clientWrapper{
		GetCurrentLoggedInUserFunc: v3c.V3.GetCurrentLoggedInUser,
		GetPrismCentralVersionFunc: func(ctx context.Context) (string, error) {
			pcInfo, err := v3c.V3.GetPrismCentral(ctx)
			if err != nil {
				return "", err
			}

			if pcInfo == nil || pcInfo.Resources == nil || pcInfo.Resources.Version == nil {
				return "", fmt.Errorf("failed to get Prism Central version: API did not return the PC version")
			}

			return *pcInfo.Resources.Version, nil
		},
		GetImageByIdFunc: func(uuid *string, args ...map[string]interface{}) (*vmmv4.GetImageApiResponse, error) {
			if uuid == nil {
				return nil, fmt.Errorf("uuid cannot be nil")
			}
			image, err := convergedc.Images.Get(context.Background(), *uuid)
			if err != nil {
				return nil, err
			}
			resp := &vmmv4.GetImageApiResponse{}
			resp.Data = vmmv4.NewOneOfGetImageApiResponseData()
			if err := resp.Data.SetValue(image); err != nil {
				return nil, fmt.Errorf("failed to set image response data: %w", err)
			}
			return resp, nil
		},
		ListImagesFunc: func(
			page_ *int,
			limit_ *int,
			filter_ *string,
			orderby_ *string,
			select_ *string,
			args ...map[string]interface{},
		) (*vmmv4.ListImagesApiResponse, error) {
			opts := buildODataOptions(page_, limit_, filter_, orderby_, select_)
			images, err := convergedc.Images.List(context.Background(), opts...)
			if err != nil {
				return nil, err
			}
			resp := &vmmv4.ListImagesApiResponse{}
			resp.Data = vmmv4.NewOneOfListImagesApiResponseData()
			if err := resp.Data.SetValue(images); err != nil {
				return nil, fmt.Errorf("failed to set images response data: %w", err)
			}
			return resp, nil
		},
		GetClusterByIdFunc: func(uuid *string, args ...map[string]interface{}) (*clustermgmtv4.GetClusterApiResponse, error) {
			if uuid == nil {
				return nil, fmt.Errorf("uuid cannot be nil")
			}
			cluster, err := convergedc.Clusters.Get(context.Background(), *uuid)
			if err != nil {
				return nil, err
			}
			resp := &clustermgmtv4.GetClusterApiResponse{}
			resp.Data = clustermgmtv4.NewOneOfGetClusterApiResponseData()
			if err := resp.Data.SetValue(cluster); err != nil {
				return nil, fmt.Errorf("failed to set cluster response data: %w", err)
			}
			return resp, nil
		},
		ListClustersFunc: func(
			page_, limit_ *int,
			filter_, orderby_, apply_, select_ *string,
			args ...map[string]interface{},
		) (*clustermgmtv4.ListClustersApiResponse, error) {
			opts := buildODataOptions(page_, limit_, filter_, orderby_, select_)
			if apply_ != nil && *apply_ != "" {
				opts = append(opts, converged.WithApply(*apply_))
			}
			clusters, err := convergedc.Clusters.List(context.Background(), opts...)
			if err != nil {
				return nil, err
			}
			resp := &clustermgmtv4.ListClustersApiResponse{}
			resp.Data = clustermgmtv4.NewOneOfListClustersApiResponseData()
			if err := resp.Data.SetValue(clusters); err != nil {
				return nil, fmt.Errorf("failed to set clusters response data: %w", err)
			}
			return resp, nil
		},
		ListStorageContainersFunc: func(
			page_ *int,
			limit_ *int,
			filter_ *string,
			orderby_ *string,
			select_ *string,
			args ...map[string]interface{},
		) (*clustermgmtv4.ListStorageContainersApiResponse, error) {
			opts := buildODataOptions(page_, limit_, filter_, orderby_, select_)
			containers, err := convergedc.StorageContainers.List(context.Background(), opts...)
			if err != nil {
				return nil, err
			}
			resp := &clustermgmtv4.ListStorageContainersApiResponse{}
			resp.Data = clustermgmtv4.NewOneOfListStorageContainersApiResponseData()
			if err := resp.Data.SetValue(containers); err != nil {
				return nil, fmt.Errorf("failed to set storage containers response data: %w", err)
			}
			return resp, nil
		},
		GetSubnetByIdFunc: func(uuid *string, args ...map[string]interface{}) (*netv4.GetSubnetApiResponse, error) {
			if uuid == nil {
				return nil, fmt.Errorf("uuid cannot be nil")
			}
			subnet, err := convergedc.Subnets.Get(context.Background(), *uuid)
			if err != nil {
				return nil, err
			}
			resp := &netv4.GetSubnetApiResponse{}
			resp.Data = netv4.NewOneOfGetSubnetApiResponseData()
			if err := resp.Data.SetValue(subnet); err != nil {
				return nil, fmt.Errorf("failed to set subnet response data: %w", err)
			}
			return resp, nil
		},
		ListSubnetsFunc: func(
			page_ *int,
			limit_ *int,
			filter_ *string,
			orderby_ *string,
			expand_ *string,
			select_ *string,
			args ...map[string]interface{},
		) (*netv4.ListSubnetsApiResponse, error) {
			opts := buildODataOptions(page_, limit_, filter_, orderby_, select_)
			if expand_ != nil && *expand_ != "" {
				opts = append(opts, converged.WithExpand(*expand_))
			}
			subnets, err := convergedc.Subnets.List(context.Background(), opts...)
			if err != nil {
				return nil, err
			}
			resp := &netv4.ListSubnetsApiResponse{}
			resp.Data = netv4.NewOneOfListSubnetsApiResponseData()
			if err := resp.Data.SetValue(subnets); err != nil {
				return nil, fmt.Errorf("failed to set subnets response data: %w", err)
			}
			return resp, nil
		},
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

func (c *clientWrapper) GetPrismCentralVersion(
	ctx context.Context,
) (
	string,
	error,
) {
	return c.GetPrismCentralVersionFunc(ctx)
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
