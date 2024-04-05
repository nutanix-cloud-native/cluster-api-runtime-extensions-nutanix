// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package iaminstanceprofile

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestIAMInstnaceProfilePatch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IAMInstanceProfile patches for ControlPlane and Workers suite")
}
