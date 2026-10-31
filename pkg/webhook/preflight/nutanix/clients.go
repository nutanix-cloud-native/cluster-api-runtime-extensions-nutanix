// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	clustermgmtv4ahv "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/ahv/config"
	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	multidomainv4 "github.com/nutanix/ntnx-api-golang-clients/multidomain-go-client/v4/models/multidomain/v4/config"
	netv4 "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"
	prismv4 "github.com/nutanix/ntnx-api-golang-clients/prism-go-client/v4/models/prism/v4/config"
	vmmv4ahv "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/ahv/config"
	vmmv4 "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/content"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	"github.com/nutanix-cloud-native/prism-go-client/converged"
	v4Converged "github.com/nutanix-cloud-native/prism-go-client/converged/v4"
	prismtypes "github.com/nutanix-cloud-native/prism-go-client/environment/types"
	v3 "github.com/nutanix-cloud-native/prism-go-client/v3"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
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
	ListImagesByFilter(
		ctx context.Context,
		filter string,
	) ([]vmmv4.Image, error)

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
	ListSubnetsByFilter(
		ctx context.Context,
		filter string,
	) ([]netv4.Subnet, error)

	GetVMProfileById(
		ctx context.Context,
		uuid *string,
		args ...map[string]any,
	) (
		*vmmv4ahv.GetVmProfileApiResponse, error,
	)

	ListVMProfiles(
		ctx context.Context,
		page_ *int,
		limit_ *int,
		filter_ *string,
		orderby_ *string,
		select_ *string,
		args ...map[string]any,
	) (
		*vmmv4ahv.ListVmProfilesApiResponse, error,
	)
	ListVMProfilesByFilter(
		ctx context.Context,
		filter string,
	) ([]vmmv4ahv.VmProfile, error)
	ListCategories(
		ctx context.Context,
		page_ *int,
		limit_ *int,
		filter_ *string,
		orderby_ *string,
		select_ *string,
		args ...map[string]any,
	) (
		*prismv4.ListCategoriesApiResponse, error,
	)
	ListCategoriesByFilter(
		ctx context.Context,
		filter string,
	) ([]prismv4.Category, error)

	ResolveProject(
		ctx context.Context,
		id *capxv1.NutanixResourceIdentifier,
		pcVersion string,
	) (
		*resolvedProject, error,
	)

	ResolvePCDefaultProject(ctx context.Context, pcVersion string) (*PCDefaultProject, error)
	ListResourceGroups(
		ctx context.Context,
		opts ...converged.ODataOption,
	) ([]multidomainv4.ResourceGroup, error)
	ListPrismElementsByResourceGroup(
		ctx context.Context,
		resourceGroupUUID string,
	) ([]converged.PrismElementInfo, error)
	ListStorageContainersByResourceGroup(
		ctx context.Context,
		resourceGroupUUID string,
	) ([]converged.StorageContainerInfo, error)
	ListClusterPhysicalGPUs(ctx context.Context, clusterUUID string) ([]clustermgmtv4.PhysicalGpuProfile, error)
	ListClusterVirtualGPUs(ctx context.Context, clusterUUID string) ([]clustermgmtv4.VirtualGpuProfile, error)
	ListAHVPhysicalGPUProfiles(ctx context.Context, clusterUUID string) ([]clustermgmtv4ahv.PhysicalGpuProfile, error)
	ListAHVVirtualGPUProfiles(ctx context.Context, clusterUUID string) ([]clustermgmtv4ahv.VirtualGpuProfile, error)
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
	ListImagesByFilterFunc func(
		ctx context.Context,
		filter string,
	) ([]vmmv4.Image, error)

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
	ListSubnetsByFilterFunc func(
		ctx context.Context,
		filter string,
	) ([]netv4.Subnet, error)

	GetVMProfileByIdFunc func(
		ctx context.Context,
		uuid *string,
		args ...map[string]any,
	) (
		*vmmv4ahv.GetVmProfileApiResponse, error,
	)

	ListVMProfilesFunc func(
		ctx context.Context,
		page_ *int,
		limit_ *int,
		filter_ *string,
		orderby_ *string,
		select_ *string,
		args ...map[string]any,
	) (
		*vmmv4ahv.ListVmProfilesApiResponse, error,
	)
	ListVMProfilesByFilterFunc func(
		ctx context.Context,
		filter string,
	) ([]vmmv4ahv.VmProfile, error)
	ListCategoriesFunc func(
		ctx context.Context,
		page_ *int,
		limit_ *int,
		filter_ *string,
		orderby_ *string,
		select_ *string,
		args ...map[string]any,
	) (
		*prismv4.ListCategoriesApiResponse, error,
	)
	ListCategoriesByFilterFunc func(
		ctx context.Context,
		filter string,
	) ([]prismv4.Category, error)

	ResolveProjectFunc func(
		ctx context.Context,
		id *capxv1.NutanixResourceIdentifier,
		pcVersion string,
	) (
		*resolvedProject, error,
	)

	ResolvePCDefaultProjectFunc func(ctx context.Context, pcVersion string) (*PCDefaultProject, error)
	ListResourceGroupsFunc      func(
		ctx context.Context,
		opts ...converged.ODataOption,
	) ([]multidomainv4.ResourceGroup, error)
	ListPrismElementsByResourceGroupFunc func(
		ctx context.Context,
		resourceGroupUUID string,
	) ([]converged.PrismElementInfo, error)
	ListStorageContainersByResourceGroupFunc func(
		ctx context.Context,
		resourceGroupUUID string,
	) ([]converged.StorageContainerInfo, error)
	ListClusterPhysicalGPUsFunc func(
		ctx context.Context,
		clusterUUID string,
	) ([]clustermgmtv4.PhysicalGpuProfile, error)
	ListClusterVirtualGPUsFunc func(
		ctx context.Context,
		clusterUUID string,
	) ([]clustermgmtv4.VirtualGpuProfile, error)
	ListAHVPhysicalGPUProfilesFunc func(
		ctx context.Context,
		clusterUUID string,
	) ([]clustermgmtv4ahv.PhysicalGpuProfile, error)
	ListAHVVirtualGPUProfilesFunc func(
		ctx context.Context,
		clusterUUID string,
	) ([]clustermgmtv4ahv.VirtualGpuProfile, error)
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
			APIKey:   credentials.APIKey,
		},
	}, nil
}

// NewNutanixPrismClient creates a client with optional cluster information for cache key.
// additionalTrustBundlePEM is the PEM-encoded trust bundle from clusterConfig
// field nutanix.prismCentralEndpoint.additionalTrustBundle (empty if not set).
//

func NewNutanixPrismClient(
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

	convergedc, err := getOrCreateConvergedClient(cacheParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create converged client: %w", err)
	}

	return &clientWrapper{
		ValidateCredentialsFunc:                newValidateCredentialsFunc(convergedc),
		GetPrismCentralVersionFunc:             newGetPrismCentralVersionFunc(convergedc),
		GetPrismCentralHostingClusterExtIDFunc: newGetPrismCentralHostingClusterExtIDFunc(convergedc),
		GetInterClusterRTTMillisFunc:           newGetInterClusterRTTMillisFunc(cacheParams),
		GetImageByIdFunc:                       newGetImageByIDFunc(convergedc),
		ListImagesFunc:                         newListImagesFunc(convergedc),
		ListImagesByFilterFunc: func(ctx context.Context, filter string) ([]vmmv4.Image, error) {
			return convergedc.Images.List(ctx, converged.WithFilter(filter))
		},
		GetClusterByIdFunc:        newGetClusterByIDFunc(convergedc),
		ListClustersFunc:          newListClustersFunc(convergedc),
		ListStorageContainersFunc: newListStorageContainersFunc(convergedc),
		GetSubnetByIdFunc:         newGetSubnetByIDFunc(convergedc),
		ListSubnetsFunc:           newListSubnetsFunc(convergedc),
		ListSubnetsByFilterFunc: func(ctx context.Context, filter string) ([]netv4.Subnet, error) {
			return convergedc.Subnets.List(ctx, converged.WithFilter(filter))
		},
		GetVMProfileByIdFunc: newGetVMProfileByIDFunc(convergedc),
		ListVMProfilesFunc:   newListVMProfilesFunc(convergedc),
		ListVMProfilesByFilterFunc: func(ctx context.Context, filter string) ([]vmmv4ahv.VmProfile, error) {
			return convergedc.VMProfiles.List(ctx, converged.WithFilter(filter))
		},
		ListCategoriesFunc: func(
			ctx context.Context,
			page_ *int,
			limit_ *int,
			filter_ *string,
			orderby_ *string,
			select_ *string,
			args ...map[string]any,
		) (*prismv4.ListCategoriesApiResponse, error) {
			opts := buildODataOptions(page_, limit_, filter_, orderby_, select_)
			categories, err := convergedc.Categories.List(ctx, opts...)
			if err != nil {
				return nil, err
			}
			resp := &prismv4.ListCategoriesApiResponse{}
			resp.Data = prismv4.NewOneOfListCategoriesApiResponseData()
			if err := resp.Data.SetValue(categories); err != nil {
				return nil, fmt.Errorf("failed to set categories response data: %w", err)
			}
			return resp, nil
		},
		ListCategoriesByFilterFunc: func(ctx context.Context, filter string) ([]prismv4.Category, error) {
			return convergedc.Categories.List(ctx, converged.WithFilter(filter))
		},
		ResolveProjectFunc:          newResolveProjectFunc(cacheParams),
		ResolvePCDefaultProjectFunc: newResolvePCDefaultProjectFunc(cacheParams),
		ListResourceGroupsFunc: func(
			ctx context.Context,
			opts ...converged.ODataOption,
		) ([]multidomainv4.ResourceGroup, error) {
			resourceGroups, err := convergedc.ResourceGroups.List(ctx, opts...)
			if err != nil {
				return nil, err
			}
			return resourceGroups, nil
		},
		ListPrismElementsByResourceGroupFunc: func(
			ctx context.Context,
			resourceGroupUUID string,
		) ([]converged.PrismElementInfo, error) {
			return convergedc.ResourceGroups.ListPrismElements(ctx, resourceGroupUUID)
		},
		ListStorageContainersByResourceGroupFunc: func(
			ctx context.Context,
			resourceGroupUUID string,
		) ([]converged.StorageContainerInfo, error) {
			return convergedc.ResourceGroups.ListStorageContainers(ctx, resourceGroupUUID)
		},
		ListClusterPhysicalGPUsFunc: func(
			ctx context.Context,
			clusterUUID string,
		) ([]clustermgmtv4.PhysicalGpuProfile, error) {
			return convergedc.Clusters.ListClusterPhysicalGPUs(ctx, clusterUUID)
		},
		ListClusterVirtualGPUsFunc: func(
			ctx context.Context,
			clusterUUID string,
		) ([]clustermgmtv4.VirtualGpuProfile, error) {
			return convergedc.Clusters.ListClusterVirtualGPUs(ctx, clusterUUID)
		},
		ListAHVPhysicalGPUProfilesFunc: func(
			ctx context.Context,
			clusterUUID string,
		) ([]clustermgmtv4ahv.PhysicalGpuProfile, error) {
			return convergedc.Clusters.ListAHVPhysicalGPUProfiles(ctx, clusterUUID)
		},
		ListAHVVirtualGPUProfilesFunc: func(
			ctx context.Context,
			clusterUUID string,
		) ([]clustermgmtv4ahv.VirtualGpuProfile, error) {
			return convergedc.Clusters.ListAHVVirtualGPUProfiles(ctx, clusterUUID)
		},
	}, nil
}

func newValidateCredentialsFunc(convergedc *v4Converged.Client) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		// Use Users.List() as a lightweight API call to validate credentials.
		// This is available to all users and serves the same purpose as V3's GetCurrentLoggedInUser.
		_, err := convergedc.Users.List(ctx, converged.WithLimit(1))
		return err
	}
}

func newGetPrismCentralVersionFunc(convergedc *v4Converged.Client) func(ctx context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		// Use DomainManager.GetPrismCentralVersion() as V4 equivalent to V3's GetPrismCentral().
		return convergedc.DomainManager.GetPrismCentralVersion(ctx)
	}
}

func newGetPrismCentralHostingClusterExtIDFunc(
	convergedc *v4Converged.Client,
) func(ctx context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
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
	}
}

func newGetInterClusterRTTMillisFunc(
	cacheParams *CacheParams,
) func(ctx context.Context, sourceClusterUUID, remoteClusterUUID string) (float64, bool, error) {
	return func(
		ctx context.Context,
		sourceClusterUUID string,
		remoteClusterUUID string,
	) (float64, bool, error) {
		return interClusterRTTMillis(ctx, cacheParams, sourceClusterUUID, remoteClusterUUID)
	}
}

func newGetImageByIDFunc(
	convergedc *v4Converged.Client,
) func(ctx context.Context, uuid *string, args ...map[string]any) (*vmmv4.GetImageApiResponse, error) {
	return func(
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
		if image == nil {
			return nil, fmt.Errorf("received nil image for uuid %q", *uuid)
		}
		resp := &vmmv4.GetImageApiResponse{}
		resp.Data = vmmv4.NewOneOfGetImageApiResponseData()
		if err := resp.Data.SetValue(*image); err != nil {
			return nil, fmt.Errorf("failed to set image response data: %w", err)
		}
		return resp, nil
	}
}

func newListImagesFunc(
	convergedc *v4Converged.Client,
) func(
	ctx context.Context,
	page_, limit_ *int,
	filter_, orderby_, select_ *string,
	args ...map[string]any,
) (*vmmv4.ListImagesApiResponse, error) {
	return func(
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
	}
}

func newGetClusterByIDFunc(
	convergedc *v4Converged.Client,
) func(ctx context.Context, uuid *string, args ...map[string]any) (*clustermgmtv4.GetClusterApiResponse, error) {
	return func(
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
	}
}

func newListClustersFunc(
	convergedc *v4Converged.Client,
) func(
	ctx context.Context,
	page_, limit_ *int,
	filter_, orderby_, apply_, select_ *string,
	args ...map[string]any,
) (*clustermgmtv4.ListClustersApiResponse, error) {
	return func(
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
	}
}

func newListStorageContainersFunc(
	convergedc *v4Converged.Client,
) func(
	ctx context.Context,
	page_, limit_ *int,
	filter_, orderby_, select_ *string,
	args ...map[string]any,
) (*clustermgmtv4.ListStorageContainersApiResponse, error) {
	return func(
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
	}
}

func newGetSubnetByIDFunc(
	convergedc *v4Converged.Client,
) func(ctx context.Context, uuid *string, args ...map[string]any) (*netv4.GetSubnetApiResponse, error) {
	return func(
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
		if subnet == nil {
			return nil, fmt.Errorf("received nil subnet response for uuid %q", *uuid)
		}
		resp := &netv4.GetSubnetApiResponse{}
		resp.Data = netv4.NewOneOfGetSubnetApiResponseData()
		// The OneOf wrapper expects netv4.Subnet (value), while converged client returns *netv4.Subnet.
		if err := resp.Data.SetValue(*subnet); err != nil {
			return nil, fmt.Errorf("failed to set subnet response data: %w", err)
		}
		return resp, nil
	}
}

func newListSubnetsFunc(
	convergedc *v4Converged.Client,
) func(
	ctx context.Context,
	page_, limit_ *int,
	filter_, orderby_, expand_, select_ *string,
	args ...map[string]any,
) (*netv4.ListSubnetsApiResponse, error) {
	return func(
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
	}
}

func newGetVMProfileByIDFunc(
	convergedc *v4Converged.Client,
) func(ctx context.Context, uuid *string, args ...map[string]any) (*vmmv4ahv.GetVmProfileApiResponse, error) {
	return func(
		ctx context.Context,
		uuid *string,
		args ...map[string]any,
	) (*vmmv4ahv.GetVmProfileApiResponse, error) {
		if uuid == nil {
			return nil, fmt.Errorf("uuid cannot be nil")
		}
		vmProfile, err := convergedc.VMProfiles.Get(ctx, *uuid)
		if err != nil {
			return nil, err
		}
		resp := &vmmv4ahv.GetVmProfileApiResponse{}
		resp.Data = vmmv4ahv.NewOneOfGetVmProfileApiResponseData()
		if err := resp.Data.SetValue(vmProfile); err != nil {
			return nil, fmt.Errorf("failed to set vm profile response data: %w", err)
		}
		return resp, nil
	}
}

func newListVMProfilesFunc(
	convergedc *v4Converged.Client,
) func(
	ctx context.Context,
	page_, limit_ *int,
	filter_, orderby_, select_ *string,
	args ...map[string]any,
) (*vmmv4ahv.ListVmProfilesApiResponse, error) {
	return func(
		ctx context.Context,
		page_ *int,
		limit_ *int,
		filter_ *string,
		orderby_ *string,
		select_ *string,
		args ...map[string]any,
	) (*vmmv4ahv.ListVmProfilesApiResponse, error) {
		opts := buildODataOptions(page_, limit_, filter_, orderby_, select_)
		vmProfiles, err := convergedc.VMProfiles.List(ctx, opts...)
		if err != nil {
			return nil, err
		}
		resp := &vmmv4ahv.ListVmProfilesApiResponse{}
		resp.Data = vmmv4ahv.NewOneOfListVmProfilesApiResponseData()
		if err := resp.Data.SetValue(vmProfiles); err != nil {
			return nil, fmt.Errorf("failed to set vm profiles response data: %w", err)
		}
		return resp, nil
	}
}

func newResolveProjectFunc(
	cacheParams *CacheParams,
) func(ctx context.Context, id *capxv1.NutanixResourceIdentifier, pcVersion string) (*resolvedProject, error) {
	return func(ctx context.Context, id *capxv1.NutanixResourceIdentifier, pcVersion string) (*resolvedProject, error) {
		return resolveProject(ctx, cacheParams, id, pcVersion)
	}
}

func newResolvePCDefaultProjectFunc(
	cacheParams *CacheParams,
) func(ctx context.Context, pcVersion string) (*PCDefaultProject, error) {
	return func(ctx context.Context, pcVersion string) (*PCDefaultProject, error) {
		return resolvePCDefaultProject(ctx, cacheParams, pcVersion)
	}
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
			{Kind: new("cluster"), UUID: new(sourceClusterUUID)},
		},
		RemoteClusterReference: &v3.Reference{
			Kind: new("cluster"),
			UUID: new(remoteClusterUUID),
		},
		RemoteAvailabilityZoneReference: &v3.Reference{
			Kind: new("availability_zone"),
			UUID: new(localAZUUID),
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

func (c *clientWrapper) ListImagesByFilter(
	ctx context.Context,
	filter string,
) ([]vmmv4.Image, error) {
	return callWithContext(ctx, func() ([]vmmv4.Image, error) {
		if c.ListImagesByFilterFunc != nil {
			return c.ListImagesByFilterFunc(ctx, filter)
		}
		if c.ListImagesFunc == nil {
			return nil, fmt.Errorf("ListImagesByFilterFunc is not set")
		}
		resp, err := c.ListImagesFunc(ctx, nil, nil, &filter, nil, nil)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.GetData() == nil {
			return []vmmv4.Image{}, nil
		}
		images, ok := resp.GetData().([]vmmv4.Image)
		if !ok {
			return nil, fmt.Errorf("failed to get data returned by ListImages")
		}
		return images, nil
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

func (c *clientWrapper) ListSubnetsByFilter(
	ctx context.Context,
	filter string,
) ([]netv4.Subnet, error) {
	return callWithContext(ctx, func() ([]netv4.Subnet, error) {
		if c.ListSubnetsByFilterFunc != nil {
			return c.ListSubnetsByFilterFunc(ctx, filter)
		}
		if c.ListSubnetsFunc == nil {
			return nil, fmt.Errorf("ListSubnetsByFilterFunc is not set")
		}
		resp, err := c.ListSubnetsFunc(ctx, nil, nil, &filter, nil, nil, nil)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.GetData() == nil {
			return []netv4.Subnet{}, nil
		}
		subnets, ok := resp.GetData().([]netv4.Subnet)
		if !ok {
			return nil, fmt.Errorf("failed to get data returned by ListSubnets")
		}
		return subnets, nil
	})
}

func (c *clientWrapper) GetVMProfileById(
	ctx context.Context,
	uuid *string,
	args ...map[string]any,
) (
	*vmmv4ahv.GetVmProfileApiResponse, error,
) {
	return callWithContext(ctx, func() (*vmmv4ahv.GetVmProfileApiResponse, error) {
		return c.GetVMProfileByIdFunc(
			ctx,
			uuid,
			args...,
		)
	})
}

func (c *clientWrapper) ListVMProfiles(
	ctx context.Context,
	page_ *int,
	limit_ *int,
	filter_ *string,
	orderby_ *string,
	select_ *string,
	args ...map[string]any,
) (
	*vmmv4ahv.ListVmProfilesApiResponse, error,
) {
	return callWithContext(ctx, func() (*vmmv4ahv.ListVmProfilesApiResponse, error) {
		return c.ListVMProfilesFunc(
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

func (c *clientWrapper) ListVMProfilesByFilter(
	ctx context.Context,
	filter string,
) ([]vmmv4ahv.VmProfile, error) {
	return callWithContext(ctx, func() ([]vmmv4ahv.VmProfile, error) {
		if c.ListVMProfilesByFilterFunc != nil {
			return c.ListVMProfilesByFilterFunc(ctx, filter)
		}
		if c.ListVMProfilesFunc == nil {
			return nil, fmt.Errorf("ListVMProfilesByFilterFunc is not set")
		}
		resp, err := c.ListVMProfilesFunc(ctx, nil, nil, &filter, nil, nil)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.GetData() == nil {
			return []vmmv4ahv.VmProfile{}, nil
		}
		vmProfiles, ok := resp.GetData().([]vmmv4ahv.VmProfile)
		if !ok {
			return nil, fmt.Errorf("failed to get data returned by ListVMProfiles")
		}
		return vmProfiles, nil
	})
}

func (c *clientWrapper) ListCategories(
	ctx context.Context,
	page_ *int,
	limit_ *int,
	filter_ *string,
	orderby_ *string,
	select_ *string,
	args ...map[string]any,
) (
	*prismv4.ListCategoriesApiResponse, error,
) {
	if c.ListCategoriesFunc == nil {
		return nil, fmt.Errorf("ListCategoriesFunc is not set")
	}
	return callWithContext(ctx, func() (*prismv4.ListCategoriesApiResponse, error) {
		return c.ListCategoriesFunc(
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

func (c *clientWrapper) ListCategoriesByFilter(
	ctx context.Context,
	filter string,
) ([]prismv4.Category, error) {
	if c.ListCategoriesByFilterFunc == nil {
		return nil, fmt.Errorf("ListCategoriesByFilterFunc is not set")
	}
	return callWithContext(ctx, func() ([]prismv4.Category, error) {
		return c.ListCategoriesByFilterFunc(ctx, filter)
	})
}

func (c *clientWrapper) ResolvePCDefaultProject(ctx context.Context, pcVersion string) (*PCDefaultProject, error) {
	if c.ResolvePCDefaultProjectFunc == nil {
		return nil, fmt.Errorf("ResolvePCDefaultProjectFunc is not set")
	}
	return callWithContext(ctx, func() (*PCDefaultProject, error) {
		return c.ResolvePCDefaultProjectFunc(ctx, pcVersion)
	})
}

func (c *clientWrapper) ResolveProject(
	ctx context.Context,
	id *capxv1.NutanixResourceIdentifier,
	pcVersion string,
) (*resolvedProject, error) {
	if c.ResolveProjectFunc == nil {
		return nil, fmt.Errorf("ResolveProjectFunc is not set")
	}
	return callWithContext(ctx, func() (*resolvedProject, error) {
		return c.ResolveProjectFunc(ctx, id, pcVersion)
	})
}

func (c *clientWrapper) ListResourceGroups(
	ctx context.Context,
	opts ...converged.ODataOption,
) ([]multidomainv4.ResourceGroup, error) {
	if c.ListResourceGroupsFunc == nil {
		return nil, fmt.Errorf("ListResourceGroupsFunc is not set")
	}
	return callWithContext(ctx, func() ([]multidomainv4.ResourceGroup, error) {
		return c.ListResourceGroupsFunc(ctx, opts...)
	})
}

func (c *clientWrapper) ListPrismElementsByResourceGroup(
	ctx context.Context,
	resourceGroupUUID string,
) ([]converged.PrismElementInfo, error) {
	if c.ListPrismElementsByResourceGroupFunc == nil {
		return nil, fmt.Errorf("ListPrismElementsByResourceGroupFunc is not set")
	}
	return callWithContext(ctx, func() ([]converged.PrismElementInfo, error) {
		return c.ListPrismElementsByResourceGroupFunc(ctx, resourceGroupUUID)
	})
}

func (c *clientWrapper) ListStorageContainersByResourceGroup(
	ctx context.Context,
	resourceGroupUUID string,
) ([]converged.StorageContainerInfo, error) {
	if c.ListStorageContainersByResourceGroupFunc == nil {
		return nil, fmt.Errorf("ListStorageContainersByResourceGroupFunc is not set")
	}
	return callWithContext(ctx, func() ([]converged.StorageContainerInfo, error) {
		return c.ListStorageContainersByResourceGroupFunc(ctx, resourceGroupUUID)
	})
}

func (c *clientWrapper) ListClusterPhysicalGPUs(
	ctx context.Context,
	clusterUUID string,
) ([]clustermgmtv4.PhysicalGpuProfile, error) {
	if c.ListClusterPhysicalGPUsFunc == nil {
		return nil, fmt.Errorf("ListClusterPhysicalGPUsFunc is not set")
	}
	return callWithContext(ctx, func() ([]clustermgmtv4.PhysicalGpuProfile, error) {
		return c.ListClusterPhysicalGPUsFunc(ctx, clusterUUID)
	})
}

func (c *clientWrapper) ListClusterVirtualGPUs(
	ctx context.Context,
	clusterUUID string,
) ([]clustermgmtv4.VirtualGpuProfile, error) {
	if c.ListClusterVirtualGPUsFunc == nil {
		return nil, fmt.Errorf("ListClusterVirtualGPUsFunc is not set")
	}
	return callWithContext(ctx, func() ([]clustermgmtv4.VirtualGpuProfile, error) {
		return c.ListClusterVirtualGPUsFunc(ctx, clusterUUID)
	})
}

func (c *clientWrapper) ListAHVPhysicalGPUProfiles(
	ctx context.Context,
	clusterUUID string,
) ([]clustermgmtv4ahv.PhysicalGpuProfile, error) {
	if c.ListAHVPhysicalGPUProfilesFunc == nil {
		return nil, fmt.Errorf("ListAHVPhysicalGPUProfilesFunc is not set")
	}
	return callWithContext(ctx, func() ([]clustermgmtv4ahv.PhysicalGpuProfile, error) {
		return c.ListAHVPhysicalGPUProfilesFunc(ctx, clusterUUID)
	})
}

func (c *clientWrapper) ListAHVVirtualGPUProfiles(
	ctx context.Context,
	clusterUUID string,
) ([]clustermgmtv4ahv.VirtualGpuProfile, error) {
	if c.ListAHVVirtualGPUProfilesFunc == nil {
		return nil, fmt.Errorf("ListAHVVirtualGPUProfilesFunc is not set")
	}
	return callWithContext(ctx, func() ([]clustermgmtv4ahv.VirtualGpuProfile, error) {
		return c.ListAHVVirtualGPUProfilesFunc(ctx, clusterUUID)
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
