package placementgroupnfd

import (
	_ "embed"
)

var (
	//go:embed embedded/placementgroup_discovery.sh
	PlacementgroupDiscoveryScript []byte

	PlacementGroupDiscoveryScriptFileOnRemote = "/etc/kubernetes/node-feature-discovery/source.d/placementgroup_discovery.sh"
)
