// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAMIPatch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils")
}
