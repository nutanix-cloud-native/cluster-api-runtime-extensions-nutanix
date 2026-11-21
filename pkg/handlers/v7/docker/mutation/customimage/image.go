// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package customimage

import (
	"github.com/blang/semver/v4"
	capdv1beta2 "sigs.k8s.io/cluster-api/test/infrastructure/docker/api/v1beta2"
)

const (
	dockerMachineTemplateKind = "DockerMachineTemplate"
	devMachineTemplateKind    = "DevMachineTemplate"

	mesosphereKinDImageRepository = "ghcr.io/mesosphere/kind-node"
	nutanixKinDImageRepository    = "ghcr.io/nutanix-cloud-native/kind-node"
)

// defaultKinDImageRepository returns the container image repository for default
// CAPD machine template custom images. mesosphere/kind-node is used through
// 1.35.x; Kubernetes 1.36+ uses ghcr.io/nutanix-cloud-native/kind-node.
func defaultKinDImageRepository(kubernetesVersion string) string {
	v, err := semver.ParseTolerant(kubernetesVersion)
	if err != nil {
		return mesosphereKinDImageRepository
	}
	if v.Major > 1 || (v.Major == 1 && v.Minor >= 36) {
		return nutanixKinDImageRepository
	}
	return mesosphereKinDImageRepository
}

func defaultKinDImage(kubernetesVersion string) string {
	return defaultKinDImageRepository(kubernetesVersion) + ":" + kubernetesVersion
}

// setDevMachineTemplateCustomImage sets the custom image on the docker backend of
// a DevMachineTemplate, creating the backend spec if the template has none yet.
func setDevMachineTemplateCustomImage(obj *capdv1beta2.DevMachineTemplate, image string) {
	if obj.Spec.Template.Spec.Backend.Docker == nil {
		obj.Spec.Template.Spec.Backend.Docker = &capdv1beta2.DockerMachineBackendSpec{}
	}
	obj.Spec.Template.Spec.Backend.Docker.CustomImage = image
}
