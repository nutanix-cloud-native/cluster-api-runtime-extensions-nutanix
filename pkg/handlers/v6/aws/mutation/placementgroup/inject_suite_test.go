// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package placementgroup

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestInstanceTypePatch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PlacementGroup patches for ControlPlane and Workers suite")
}
