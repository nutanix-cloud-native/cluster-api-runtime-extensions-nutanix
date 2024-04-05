// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package customimage

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCustomImagePatch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Docker CustomImage patches for ControlPlane and Workers suite")
}
