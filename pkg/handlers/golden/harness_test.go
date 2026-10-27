// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package golden

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadInput(t *testing.T) {
	t.Parallel()

	in, err := LoadInput("testdata/corpus/docker/docker-cluster-cilium-helm-addon.yaml")
	require.NoError(t, err)
	assert.Equal(t, "docker-quick-start", in.ClusterClass.Name)
	assert.Equal(t, "golden", in.Cluster.Name)
	assert.Len(t, in.Templates, 5)
	assert.Contains(t, in.Templates, TemplateKey{Kind: "DockerClusterTemplate", Name: "docker-quick-start-cluster"})
	assert.Empty(t, in.Fixtures)

	in, err = LoadInput("testdata/corpus/nutanix/nutanix-cluster-cilium-helm-addon.yaml")
	require.NoError(t, err)
	assert.Len(t, in.Templates, 5)
	assert.Len(t, in.Fixtures, 4)
}
