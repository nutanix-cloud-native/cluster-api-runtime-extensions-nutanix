// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
)

const (
	nkpClusterClassPrefix = "nkp-"
	nkp219Major           = 2
	nkp219Minor           = 19
)

// IsNKP219ClusterClass reports whether className is an NKP ClusterClass for the 2.19 release line.
func IsNKP219ClusterClass(className string) (bool, error) {
	if !strings.HasPrefix(className, nkpClusterClassPrefix) {
		return false, nil
	}

	versionStart := strings.LastIndex(className, "-v")
	if versionStart == -1 {
		return false, nil
	}
	version, err := semver.ParseTolerant(className[versionStart+1:])
	if err != nil {
		return false, fmt.Errorf("invalid ClusterClass version in %q: %w", className, err)
	}
	return version.Major == nkp219Major && version.Minor == nkp219Minor, nil
}
