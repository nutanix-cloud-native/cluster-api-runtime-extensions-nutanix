// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestKubeletConfiguration_IsEmpty(t *testing.T) {
	assert.True(t, (*KubeletConfiguration)(nil).IsEmpty())

	empty := &KubeletConfiguration{}
	assert.True(t, empty.IsEmpty())

	withField := &KubeletConfiguration{MaxPods: new(int32(110))}
	assert.False(t, withField.IsEmpty())

	withMinAge := &KubeletConfiguration{
		ImageMinimumGCAge: &metav1.Duration{Duration: 5 * time.Minute},
	}
	assert.False(t, withMinAge.IsEmpty())

	withMaxAge := &KubeletConfiguration{
		ImageMaximumGCAge: &metav1.Duration{Duration: 48 * time.Hour},
	}
	assert.False(t, withMaxAge.IsEmpty())
}

func TestKubeletConfiguration_IsEmpty_AutomaticReservations(t *testing.T) {
	cfg := &KubeletConfiguration{
		AutomaticReservations: &AutomaticReservations{
			Profile: ReservationProfileCapacityTiered,
		},
	}
	assert.False(t, cfg.IsEmpty())
}
