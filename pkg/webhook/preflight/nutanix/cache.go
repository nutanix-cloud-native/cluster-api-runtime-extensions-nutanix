package nutanix

import (
	v4Converged "github.com/nutanix-cloud-native/prism-go-client/converged/v4"
	"github.com/nutanix-cloud-native/prism-go-client/environment/types"
	v3 "github.com/nutanix-cloud-native/prism-go-client/v3"
)

// NutanixClientCache is the cache of prism clients to be shared across the different controllers.
//
//nolint:gochecknoglobals // Client cache must be a package-level singleton for connection pooling
var NutanixClientCache = v3.NewClientCache(v3.WithSessionAuth(true))

// NutanixConvergedClientV4Cache is the cache of prism clients to be shared across the different controllers.
//
//nolint:gochecknoglobals // Client cache must be a package-level singleton for connection pooling
var NutanixConvergedClientV4Cache = v4Converged.NewClientCache()

// CacheParams is the struct that implements ClientCacheParams interface from prism-go-client.
type CacheParams struct {
	PrismManagementEndpoint *types.ManagementEndpoint
}

// ManagementEndpoint returns the management endpoint of the NutanixCluster CR.
func (c *CacheParams) ManagementEndpoint() types.ManagementEndpoint {
	return *c.PrismManagementEndpoint
}

// Key returns a unique key for the client cache based on the management endpoint.
func (c *CacheParams) Key() string {
	return c.PrismManagementEndpoint.Address.String()
}
