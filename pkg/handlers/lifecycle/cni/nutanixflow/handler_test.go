// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanixflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImagePullSecretNamespaces(t *testing.T) {
	t.Parallel()

	// Flow CNI 1.1.0 deploys private images into three namespaces. The handler
	// copies imagePullCredentials into each so kubelet can pull them.
	assert.ElementsMatch(t, []string{
		"flow-cni-system",
		"flow-cns-system",
		"ovn-kubernetes",
	}, imagePullSecretNamespaces)
}
