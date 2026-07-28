// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package identityref

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEKSIdentityRefPatch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EKS IdentityRef patches for ControlPlane suite")
}
