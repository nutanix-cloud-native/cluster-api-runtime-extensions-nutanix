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
	k8stypes "k8s.io/apimachinery/pkg/types"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	"github.com/nutanix-cloud-native/prism-go-client/converged"
	"github.com/nutanix-cloud-native/prism-go-client/environment/types"
)

// client contains methods to interact with Nutanix Prism converged v4 client.
type client interface {
	// ValidateCredentials validates credentials by making a lightweight API call.
	// This replaces the V3 GetCurrentLoggedInUser() for credential validation.
	ValidateCredentials(
		ctx context.Context,
	) error

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

// clientWrapper implements the client interface and wraps converged v4 client.
type clientWrapper struct {
	ValidateCredentialsFunc func(
		ctx context.Context,
	) error

	GetPrismCentralVersionFunc func(
		ctx context.Context,
	) (
		string, error,
	)

	GetImageByIdFunc func(
		ctx context.Context,
		uuid *string,
		args ...map[string]interface{},
	) (
		*vmmv4.GetImageApiResponse, error,
	)

	ListImagesFunc func(
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

	GetClusterByIdFunc func(
		ctx context.Context,
		uuid *string,
		args ...map[string]interface{},
	) (
		*clustermgmtv4.GetClusterApiResponse, error,
	)

	ListClustersFunc func(
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
	ListStorageContainersFunc func(
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

	GetSubnetByIdFunc func(
		ctx context.Context,
		uuid *string,
		args ...map[string]interface{},
	) (
		*netv4.GetSubnetApiResponse, error,
	)

	ListSubnetsFunc func(
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

// newClient creates a client with optional cluster information for cache key.
func newClient(
	credentials prismgoclient.Credentials, //nolint:gocritic // hugeParam is fine
	clusterNamespacedName k8stypes.NamespacedName,
) (client, error) {
	endpoint, err := buildManagementEndpoint(&credentials)
	if err != nil {
		return nil, fmt.Errorf("failed to build management endpoint: %w", err)
	}
	cacheParams := &CacheParams{
		ClusterNamespacedName:   clusterNamespacedName,
		PrismManagementEndpoint: endpoint,
	}

	convergedc, err := NutanixConvergedClientV4Cache.GetOrCreate(cacheParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create converged client: %w", err)
	}

	return &clientWrapper{
		ValidateCredentialsFunc: func(ctx context.Context) error {
			// Use Users.List() as a lightweight API call to validate credentials.
			// This is available to all users and serves the same purpose as V3's GetCurrentLoggedInUser.
			_, err := convergedc.Users.List(ctx, converged.WithLimit(1))
			return err
		},
		GetPrismCentralVersionFunc: func(ctx context.Context) (string, error) {
			// Use DomainManager.GetPrismCentralVersion() as V4 equivalent to V3's GetPrismCentral().
			return convergedc.DomainManager.GetPrismCentralVersion(ctx)
		},
		GetImageByIdFunc: func(
			ctx context.Context,
			uuid *string,
			args ...map[string]interface{},
		) (*vmmv4.GetImageApiResponse, error) {
			if uuid == nil {
				return nil, fmt.Errorf("uuid cannot be nil")
			}
			image, err := convergedc.Images.Get(ctx, *uuid)
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
			ctx context.Context,
			page_ *int,
			limit_ *int,
			filter_ *string,
			orderby_ *string,
			select_ *string,
			args ...map[string]interface{},
		) (*vmmv4.ListImagesApiResponse, error) {
			opts := buildODataOptions(page_, limit_, filter_, orderby_, select_)
			images, err := convergedc.Images.List(ctx, opts...)
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
		GetClusterByIdFunc: func(
			ctx context.Context,
			uuid *string,
			args ...map[string]interface{},
		) (*clustermgmtv4.GetClusterApiResponse, error) {
			if uuid == nil {
				return nil, fmt.Errorf("uuid cannot be nil")
			}
			cluster, err := convergedc.Clusters.Get(ctx, *uuid)
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
			ctx context.Context,
			page_, limit_ *int,
			filter_, orderby_, apply_, select_ *string,
			args ...map[string]interface{},
		) (*clustermgmtv4.ListClustersApiResponse, error) {
			opts := buildODataOptions(page_, limit_, filter_, orderby_, select_)
			if apply_ != nil && *apply_ != "" {
				opts = append(opts, converged.WithApply(*apply_))
			}
			clusters, err := convergedc.Clusters.List(ctx, opts...)
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
			ctx context.Context,
			page_ *int,
			limit_ *int,
			filter_ *string,
			orderby_ *string,
			select_ *string,
			args ...map[string]interface{},
		) (*clustermgmtv4.ListStorageContainersApiResponse, error) {
			opts := buildODataOptions(page_, limit_, filter_, orderby_, select_)
			containers, err := convergedc.StorageContainers.List(ctx, opts...)
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
		GetSubnetByIdFunc: func(
			ctx context.Context,
			uuid *string,
			args ...map[string]interface{},
		) (*netv4.GetSubnetApiResponse, error) {
			if uuid == nil {
				return nil, fmt.Errorf("uuid cannot be nil")
			}
			subnet, err := convergedc.Subnets.Get(ctx, *uuid)
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
			ctx context.Context,
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
			subnets, err := convergedc.Subnets.List(ctx, opts...)
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

func (c *clientWrapper) ValidateCredentials(
	ctx context.Context,
) error {
	return c.ValidateCredentialsFunc(ctx)
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
			ctx,
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
			ctx,
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
			ctx,
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
			ctx,
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
			ctx,
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
			ctx,
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
			ctx,
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
