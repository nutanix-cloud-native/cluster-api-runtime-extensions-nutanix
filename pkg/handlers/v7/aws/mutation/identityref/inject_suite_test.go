// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package identityref

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAWSIdentityRefPatch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWS IdentityRef patches for ControlPlane suite")
}
