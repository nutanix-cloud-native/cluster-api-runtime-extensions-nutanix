// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestKubeletConfiguration_IsEmpty(t *testing.T) {
	assert.True(t, (*KubeletConfiguration)(nil).IsEmpty())

	empty := &KubeletConfiguration{}
	assert.True(t, empty.IsEmpty())

	withField := &KubeletConfiguration{MaxPods: ptr.To(int32(110))}
	assert.False(t, withField.IsEmpty())
}

func TestKubeletConfiguration_IsEmpty_AutomaticReservations(t *testing.T) {
	cfg := &KubeletConfiguration{
		AutomaticReservations: &AutomaticReservations{
			Profile: ReservationProfileCapacityTiered,
		},
	}
	assert.False(t, cfg.IsEmpty())
}
