// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package versions

import (
	"fmt"

	"github.com/blang/semver/v4"
)

// GetCoreKubernetesVersion returns the Kubernetes version in the format "vX.Y.Z", stripping any build metadata.
func GetCoreKubernetesVersion(kubernetesVersion string) (string, error) {
	ver, err := semver.ParseTolerant(kubernetesVersion)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("v%d.%d.%d", ver.Major, ver.Minor, ver.Patch), nil
}
