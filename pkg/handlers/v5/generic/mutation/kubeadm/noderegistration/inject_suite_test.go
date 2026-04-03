// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package noderegistration

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestNodeRegistrationPatch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NodeRegistration patches for ControlPlane and Workers suite")
}
