// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package kubeletconfiguration

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestKubeletConfigurationPatch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "KubeletConfiguration patches for ControlPlane and Workers suite")
}
