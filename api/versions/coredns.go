// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Code generated by script; DO NOT EDIT. Run 'make coredns.sync' instead

package versions

import (
	"fmt"
	"maps"

	"github.com/blang/semver/v4"
)

// Kubernetes versions
const (
	Kubernetes_V1_22 = "v1.22"
	Kubernetes_V1_23 = "v1.23"
	Kubernetes_V1_24 = "v1.24"
	Kubernetes_V1_25 = "v1.25"
	Kubernetes_V1_26 = "v1.26"
	Kubernetes_V1_27 = "v1.27"
	Kubernetes_V1_28 = "v1.28"
	Kubernetes_V1_29 = "v1.29"
	Kubernetes_V1_30 = "v1.30"
	Kubernetes_V1_31 = "v1.31"
	Kubernetes_V1_32 = "v1.32"
)

// CoreDNS versions
const (
	CoreDNS_V1_8_4  = "v1.8.4"
	CoreDNS_V1_8_6  = "v1.8.6"
	CoreDNS_V1_9_3  = "v1.9.3"
	CoreDNS_V1_10_1 = "v1.10.1"
	CoreDNS_V1_11_1 = "v1.11.1"
	CoreDNS_V1_11_3 = "v1.11.3"
)

// kubernetesToCoreDNSVersion maps Kubernetes versions to CoreDNS versions.
// This map is unexported to prevent external modification.
var kubernetesToCoreDNSVersion = map[string]string{
	Kubernetes_V1_22: CoreDNS_V1_8_4,
	Kubernetes_V1_23: CoreDNS_V1_8_6,
	Kubernetes_V1_24: CoreDNS_V1_8_6,
	Kubernetes_V1_25: CoreDNS_V1_9_3,
	Kubernetes_V1_26: CoreDNS_V1_9_3,
	Kubernetes_V1_27: CoreDNS_V1_10_1,
	Kubernetes_V1_28: CoreDNS_V1_10_1,
	Kubernetes_V1_29: CoreDNS_V1_11_1,
	Kubernetes_V1_30: CoreDNS_V1_11_3,
	Kubernetes_V1_31: CoreDNS_V1_11_3,
	Kubernetes_V1_32: CoreDNS_V1_11_3,
}

// GetCoreDNSVersion returns the CoreDNS version for a given Kubernetes version.
// It accepts versions with or without the "v" prefix and handles full semver versions.
// The function maps based on the major and minor versions (e.g., "v1.27").
// If the Kubernetes version is not found, it returns an empty string and false.
func GetCoreDNSVersion(kubernetesVersion string) (string, bool) {
	// Parse the version using semver
	v, err := semver.ParseTolerant(kubernetesVersion)
	if err != nil {
		return "", false
	}

	// Construct "vMAJOR.MINOR" format
	majorMinor := fmt.Sprintf("v%d.%d", v.Major, v.Minor)

	// Lookup the CoreDNS version using the major and minor version
	version, found := kubernetesToCoreDNSVersion[majorMinor]
	return version, found
}

// GetKubernetesToCoreDNSVersionMap returns a copy of the Kubernetes to CoreDNS version mapping.
// The map keys are Kubernetes versions in "vMAJOR.MINOR" format.
func GetKubernetesToCoreDNSVersionMap() map[string]string {
	return maps.Clone(kubernetesToCoreDNSVersion)
}
