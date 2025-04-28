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

func TestReturnsCorrectMappingForGetKubernetesToCoreDNSVersionMap(t *testing.T) {
	mapping := GetKubernetesToCoreDNSVersionMap()
	expected := map[string]string{
		"v1.22": "v1.8.4",
		"v1.23": "v1.8.6",
		"v1.24": "v1.8.6",
		"v1.25": "v1.9.3",
		"v1.26": "v1.9.3",
		"v1.27": "v1.10.1",
		"v1.28": "v1.10.1",
		"v1.29": "v1.11.1",
		"v1.30": "v1.11.3",
		"v1.31": "v1.11.3",
		"v1.32": "v1.11.3",
		"v1.33": "v1.12.0",
	}
	assert.Equal(t, expected, mapping)
}
