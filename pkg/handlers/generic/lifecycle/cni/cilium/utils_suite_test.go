// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cilium

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestCiliumHandler is the entrypoint for integration (envtest) tests.
func TestCiliumHandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cilium")
}
