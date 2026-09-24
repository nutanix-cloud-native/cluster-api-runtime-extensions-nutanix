// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package customimage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultKinDImageRepository(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		want    string
	}{
		{name: "1.35", version: "v1.35.3", want: mesosphereKinDImageRepository},
		{name: "1.36", version: "v1.36.1", want: mesosphereKinDImageRepository},
		{name: "1.37", version: "v1.37.0", want: kindestKinDImageRepository},
		{name: "1.37 without v", version: "1.37.0", want: kindestKinDImageRepository},
		{name: "invalid", version: "not-a-version", want: mesosphereKinDImageRepository},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, defaultKinDImageRepository(tt.version))
			if tt.version != "not-a-version" {
				assert.Equal(t, tt.want+":"+tt.version, defaultKinDImage(tt.version))
			}
		})
	}
}
