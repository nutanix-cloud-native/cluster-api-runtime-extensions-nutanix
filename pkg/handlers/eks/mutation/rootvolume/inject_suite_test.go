// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package rootvolume

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRootVolumePatch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EKS root volume patches for Workers suite")
}
