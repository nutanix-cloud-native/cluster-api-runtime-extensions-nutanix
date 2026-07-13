// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package versions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReturnsCorrectCoreDNSVersionForValidKubernetesVersion(t *testing.T) {
	version, found := GetCoreDNSVersion("v1.27")
	assert.True(t, found)
	assert.Equal(t, "v1.10.1", version)
}

func TestReturnsCorrectCoreDNSVersionForValidKubernetesVersionWithoutVPrefix(t *testing.T) {
	version, found := GetCoreDNSVersion("1.27")
	assert.True(t, found)
	assert.Equal(t, "v1.10.1", version)
}

func TestReturnsFalseForInvalidKubernetesVersion(t *testing.T) {
	version, found := GetCoreDNSVersion("v2.99")
	assert.False(t, found)
	assert.Empty(t, version)
}

func TestReturnsFalseForMalformedKubernetesVersion(t *testing.T) {
	version, found := GetCoreDNSVersion("invalid-version")
	assert.False(t, found)
	assert.Empty(t, version)
}

func TestReturnsCopyForGetKubernetesToCoreDNSVersionMap(t *testing.T) {
	mapping := GetKubernetesToCoreDNSVersionMap()
	mapping["v1.27"] = "modified"

	version, found := GetCoreDNSVersion("v1.27")
	assert.True(t, found)
	assert.Equal(t, "v1.10.1", version)
}
