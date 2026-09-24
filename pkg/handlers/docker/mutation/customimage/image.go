// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package customimage

import (
	"github.com/blang/semver/v4"
)

const (
	mesosphereKinDImageRepository = "ghcr.io/mesosphere/kind-node"
	kindestKinDImageRepository    = "kindest/node"
)

// defaultKinDImageRepository returns the container image repository for default
// DockerMachineTemplate custom images. mesosphere/kind-node currently mirrors
// through 1.36.x; Kubernetes 1.37+ uses upstream kindest/node.
func defaultKinDImageRepository(kubernetesVersion string) string {
	v, err := semver.ParseTolerant(kubernetesVersion)
	if err != nil {
		return mesosphereKinDImageRepository
	}
	if v.Major > 1 || (v.Major == 1 && v.Minor >= 37) {
		return kindestKinDImageRepository
	}
	return mesosphereKinDImageRepository
}

func defaultKinDImage(kubernetesVersion string) string {
	return defaultKinDImageRepository(kubernetesVersion) + ":" + kubernetesVersion
}
