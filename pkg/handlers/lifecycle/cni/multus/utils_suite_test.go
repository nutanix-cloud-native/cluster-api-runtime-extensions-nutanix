// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package multus

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestMultusHandler is the entrypoint for integration (envtest) tests.
func TestMultusHandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Multus Handler Integration Tests")
}

