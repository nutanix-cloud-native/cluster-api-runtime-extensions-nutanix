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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	"github.com/nutanix-cloud-native/prism-go-client/converged"
	prismtypes "github.com/nutanix-cloud-native/prism-go-client/environment/types"
	v3 "github.com/nutanix-cloud-native/prism-go-client/v3"
)

// localAvailabilityZoneManagementPlaneType is the management plane type that
// identifies the Availability Zone local to the connected Prism Central.
const localAvailabilityZoneManagementPlaneType = "Local"

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

	// GetPrismCentralHostingClusterExtID returns the ExtId of the Prism Element
	// cluster that hosts Prism Central. It returns an empty string when the
	// hosting cluster cannot be determined.
	GetPrismCentralHostingClusterExtID(
		ctx context.Context,
	) (
		string,
		error,
	)

	// GetInterClusterRTTMillis returns the network round-trip time, in
	// milliseconds, between two Prism Element clusters managed by this Prism
	// Central, as reported by the synchronous-replication-capable API. The
	// returned boolean is false when the round-trip time could not be
	// determined (for example, when the two clusters are not synchronous
	// replication capable).
	GetInterClusterRTTMillis(
		ctx context.Context,
		sourceClusterUUID string,
		remoteClusterUUID string,
	) (
		float64,
		bool,
		error,
	)

	GetImageById(
		ctx context.Context,
		uuid *string,
		args ...map[string]any,
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
		args ...map[string]any,
	) (
		*vmmv4.ListImagesApiResponse,
		error,
	)

	GetClusterById(
		ctx context.Context,
		uuid *string,
		args ...map[string]any,
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
		args ...map[string]any,
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
		args ...map[string]any,
	) (
		*clustermgmtv4.ListStorageContainersApiResponse,
		error,
	)

	GetSubnetById(
		ctx context.Context,
		uuid *string,
		args ...map[string]any,
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
		args ...map[string]any,
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

	GetPrismCentralHostingClusterExtIDFunc func(
		ctx context.Context,
	) (
		string, error,
	)

	GetInterClusterRTTMillisFunc func(
		ctx context.Context,
		sourceClusterUUID string,
		remoteClusterUUID string,
	) (
		float64, bool, error,
	)

	GetImageByIdFunc func(
		ctx context.Context,
		uuid *string,
		args ...map[string]any,
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
		args ...map[string]any,
	) (
		*vmmv4.ListImagesApiResponse,
		error,
	)

	GetClusterByIdFunc func(
		ctx context.Context,
		uuid *string,
		args ...map[string]any,
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
		args ...map[string]any,
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
		args ...map[string]any,
	) (
		*clustermgmtv4.ListStorageContainersApiResponse,
		error,
	)

	GetSubnetByIdFunc func(
		ctx context.Context,
		uuid *string,
		args ...map[string]any,
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
		args ...map[string]any,
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

// buildManagementEndpoint creates a ManagementEndpoint from credentials and optional trust bundle.
// additionalTrustBundlePEM is PEM-encoded certificate bundle (empty string if not set).
func buildManagementEndpoint(
	credentials *prismgoclient.Credentials,
	additionalTrustBundlePEM string,
) (*prismtypes.ManagementEndpoint, error) {
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

	return &prismtypes.ManagementEndpoint{
		Address:               parsedURL,
		Insecure:              credentials.Insecure,
		AdditionalTrustBundle: additionalTrustBundlePEM,
		ApiCredentials: prismtypes.ApiCredentials{
			Username: credentials.Username,
			Password: credentials.Password,
		},
	}, nil
}

// newClient creates a client with optional cluster information for cache key.
// additionalTrustBundlePEM is the PEM-encoded trust bundle from clusterConfig
// field nutanix.prismCentralEndpoint.additionalTrustBundle (empty if not set).
func newClient(
	credentials prismgoclient.Credentials, //nolint:gocritic // hugeParam is fine
	clusterNamespacedName types.NamespacedName,
	additionalTrustBundlePEM string,
) (client, error) {
	endpoint, err := buildManagementEndpoint(&credentials, additionalTrustBundlePEM)
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
		GetPrismCentralHostingClusterExtIDFunc: func(ctx context.Context) (string, error) {
			domainManagers, err := convergedc.DomainManager.List(ctx)
			if err != nil {
				return "", err
			}
			for i := range domainManagers {
				if domainManagers[i].HostingClusterExtId != nil && *domainManagers[i].HostingClusterExtId != "" {
					return *domainManagers[i].HostingClusterExtId, nil
				}
			}
			return "", nil
		},
		GetInterClusterRTTMillisFunc: func(
			ctx context.Context,
			sourceClusterUUID string,
			remoteClusterUUID string,
		) (float64, bool, error) {
			return interClusterRTTMillis(ctx, cacheParams, sourceClusterUUID, remoteClusterUUID)
		},
		GetImageByIdFunc: func(
			ctx context.Context,
			uuid *string,
			args ...map[string]any,
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
			args ...map[string]any,
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
			args ...map[string]any,
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
			args ...map[string]any,
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
			args ...map[string]any,
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
			args ...map[string]any,
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
			args ...map[string]any,
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

func (c *clientWrapper) GetPrismCentralHostingClusterExtID(
	ctx context.Context,
) (
	string,
	error,
) {
	return callWithContext(ctx, func() (string, error) {
		return c.GetPrismCentralHostingClusterExtIDFunc(ctx)
	})
}

func (c *clientWrapper) GetInterClusterRTTMillis(
	ctx context.Context,
	sourceClusterUUID string,
	remoteClusterUUID string,
) (
	rttMillis float64,
	ok bool,
	err error,
) {
	return c.GetInterClusterRTTMillisFunc(ctx, sourceClusterUUID, remoteClusterUUID)
}

// interClusterRTTMillis returns the network round-trip time, in milliseconds,
// between two Prism Element clusters, as reported by the V3
// synchronous-replication-capable API. The returned boolean is false when the
// round-trip time could not be determined.
func interClusterRTTMillis(
	ctx context.Context,
	cacheParams *CacheParams,
	sourceClusterUUID string,
	remoteClusterUUID string,
) (rttMillis float64, ok bool, err error) {
	// The synchronous-replication-capable API is only available on the V3 API,
	// so use a V3 client that shares the same endpoint and credentials as the
	// V4 client.
	v3client, err := NutanixClientCache.GetOrCreate(cacheParams)
	if err != nil {
		return 0, false, fmt.Errorf("failed to create Prism Central V3 API client: %w", err)
	}

	azResp, err := v3client.V3.ListAllAvailabilityZones(ctx, "")
	if err != nil {
		return 0, false, fmt.Errorf("failed to list availability zones: %w", err)
	}
	localAZUUID := localAvailabilityZoneUUID(azResp)
	if localAZUUID == "" {
		return 0, false, nil
	}

	input := &v3.ClusterSyncReplicationCapableInput{
		SourceClusterReferenceList: []*v3.Reference{
			{Kind: ptr.To("cluster"), UUID: ptr.To(sourceClusterUUID)},
		},
		RemoteClusterReference: &v3.Reference{
			Kind: ptr.To("cluster"),
			UUID: ptr.To(remoteClusterUUID),
		},
		RemoteAvailabilityZoneReference: &v3.Reference{
			Kind: ptr.To("availability_zone"),
			UUID: ptr.To(localAZUUID),
		},
	}
	resp, err := v3client.V3.GetSyncReplicationCapableClusters(ctx, input)
	if err != nil {
		return 0, false, fmt.Errorf("failed to check synchronous replication capability: %w", err)
	}
	if resp == nil {
		return 0, false, nil
	}
	for _, entry := range *resp {
		if entry == nil || entry.RttMsecs == nil {
			continue
		}
		rtt, err := entry.RttMsecs.Float64()
		if err != nil {
			return 0, false, fmt.Errorf("failed to parse round-trip time %q: %w", entry.RttMsecs.String(), err)
		}
		return rtt, true, nil
	}
	return 0, false, nil
}

// localAvailabilityZoneUUID returns the UUID of the Availability Zone local to
// the connected Prism Central, or an empty string when it cannot be found.
func localAvailabilityZoneUUID(resp *v3.AvailabilityZoneListResponse) string {
	if resp == nil {
		return ""
	}
	for _, az := range resp.Entities {
		if az == nil || az.Metadata == nil || az.Status == nil {
			continue
		}
		if az.Status.Resources != nil &&
			ptr.Deref(az.Status.Resources.ManagementPlaneType, "") == localAvailabilityZoneManagementPlaneType {
			return ptr.Deref(az.Metadata.UUID, "")
		}
	}
	return ""
}

func (c *clientWrapper) GetImageById(
	ctx context.Context,
	uuid *string,
	args ...map[string]any,
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
	args ...map[string]any,
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
	args ...map[string]any,
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
	args ...map[string]any,
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
	args ...map[string]any,
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
	args ...map[string]any,
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
	args ...map[string]any,
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
