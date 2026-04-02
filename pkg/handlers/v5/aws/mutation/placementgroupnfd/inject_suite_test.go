// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package placementgroupnfd

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPlacementGroupNFDPatch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWS Placement Group NFD mutator suite")
}
