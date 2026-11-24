// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolvePCDefaultProject_KnownVersionBelow76NeedsNoClient(t *testing.T) {
	t.Parallel()

	for _, pcVersion := range []string{"7.5.0", "pc.7.5.0.5", "skipped"} {
		t.Run(pcVersion, func(t *testing.T) {
			t.Parallel()
			got, err := resolvePCDefaultProject(context.Background(), nil, pcVersion)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, v3DefaultProjectName, got.Name)
			assert.Empty(t, got.UUID)
		})
	}
}

func TestResolvePCDefaultProject_EmptyVersionNeedsEndpoint(t *testing.T) {
	t.Parallel()

	got, err := resolvePCDefaultProject(context.Background(), nil, "")
	require.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "cache parameters or management endpoint is nil")
}

func TestResolvePCDefaultProject_Known76StillNeedsEndpoint(t *testing.T) {
	t.Parallel()

	got, err := resolvePCDefaultProject(context.Background(), nil, "7.6.0")
	require.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "cache parameters or management endpoint is nil")
}
