// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanixflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlowNamespaces(t *testing.T) {
	t.Parallel()

	// Flow CNI 1.1.0 deploys into three namespaces. The handler always ensures
	// they exist (including when imagePullCredentials are omitted for a
	// registry mirror), and copies credentials into each when provided.
	assert.ElementsMatch(t, []string{
		"flow-cni-system",
		"flow-cns-system",
		"ovn-kubernetes",
	}, flowNamespaces)
}
