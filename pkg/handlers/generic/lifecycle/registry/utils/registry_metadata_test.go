// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_certificateDNSNames(t *testing.T) {
	//nolint:lll // Keep long lines for readability.
	expected := []string{
		"cncf-distribution-registry-docker-registry",
		"cncf-distribution-registry-docker-registry.registry-system",
		"cncf-distribution-registry-docker-registry.registry-system.svc",
		"cncf-distribution-registry-docker-registry.registry-system.svc.cluster.local",
		"cncf-distribution-registry-docker-registry-0",
		"cncf-distribution-registry-docker-registry-0.cncf-distribution-registry-docker-registry-headless.registry-system",
		"cncf-distribution-registry-docker-registry-0.cncf-distribution-registry-docker-registry-headless.registry-system.svc",
		"cncf-distribution-registry-docker-registry-0.cncf-distribution-registry-docker-registry-headless.registry-system.svc.cluster.local",
		"cncf-distribution-registry-docker-registry-1",
		"cncf-distribution-registry-docker-registry-1.cncf-distribution-registry-docker-registry-headless.registry-system",
		"cncf-distribution-registry-docker-registry-1.cncf-distribution-registry-docker-registry-headless.registry-system.svc",
		"cncf-distribution-registry-docker-registry-1.cncf-distribution-registry-docker-registry-headless.registry-system.svc.cluster.local",
	}

	assert.Equal(
		t,
		expected,
		getCertificateDNSNames(
			"cncf-distribution-registry-docker-registry",
			"cncf-distribution-registry-docker-registry-headless",
			"registry-system",
			2,
		),
	)
}
