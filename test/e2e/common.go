//go:build e2e

// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
)

const (
	KubernetesVersion = "KUBERNETES_VERSION"
)

// CheckTestBeforeCleanup checks to see if the current running Ginkgo test failed, and prints
// a status message regarding cleanup.
func CheckTestBeforeCleanup() {
	if CurrentSpecReport().State.Is(types.SpecStateFailureStates) {
		Logf("FAILED!")
	}
	Logf("Cleaning up after \"%s\" spec", CurrentSpecReport().FullText())
}

func Byf(format string, a ...interface{}) {
	By(fmt.Sprintf(format, a...))
}
