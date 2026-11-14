// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package placementgroupnfd

import (
	_ "embed"
)

var (
	//go:embed embedded/placementgroup_discovery.sh
	PlacementgroupDiscoveryScript []byte
	//nolint:lll // this is a constant with long file path
	PlacementGroupDiscoveryScriptFileOnRemote = "/etc/kubernetes/node-feature-discovery/source.d/placementgroup_discovery.sh"
)
