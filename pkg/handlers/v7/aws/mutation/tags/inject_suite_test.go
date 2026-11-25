// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tags

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

func TestTagsPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "AWS Tags mutator suite")
}
