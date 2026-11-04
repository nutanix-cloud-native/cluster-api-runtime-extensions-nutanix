// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsNKP219ClusterClass(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		class string
		want  bool
	}{
		{name: "first release", class: "nkp-nutanix-v2.19.0", want: true},
		{name: "patch release", class: "nkp-nutanix-v2.19.3", want: true},
		{name: "development release", class: "nkp-nutanix-v2.19.0-dev", want: true},
		{name: "previous minor", class: "nkp-nutanix-v2.18.8", want: false},
		{name: "next minor", class: "nkp-nutanix-v2.20.0", want: false},
		{name: "custom class", class: "customer-v2.19.0", want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := IsNKP219ClusterClass(tt.class)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
