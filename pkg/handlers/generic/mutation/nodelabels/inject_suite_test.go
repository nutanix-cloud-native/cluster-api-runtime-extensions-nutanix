// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nodelabels

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestNodeLabelsPatch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NodeLabels patches for Workers suite")
}
