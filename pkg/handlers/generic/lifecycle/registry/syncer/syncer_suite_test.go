// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package syncer

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestSyncer is the entrypoint for integration (envtest) tests.
func TestSyncer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Syncer")
}
