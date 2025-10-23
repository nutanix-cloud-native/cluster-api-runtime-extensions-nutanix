// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package failuredomain

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestFailureDomainPatch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWS failure domain mutator suite")
}
