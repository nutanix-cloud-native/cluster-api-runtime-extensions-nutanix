// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
)

func TestIsV3DefaultProjectName(t *testing.T) {
	t.Parallel()
	assert.True(t, IsV3DefaultProjectName("_internal"))
	assert.True(t, IsV3DefaultProjectName(" _Internal "))
	assert.False(t, IsV3DefaultProjectName("manoj-proj"))
	assert.False(t, IsV3DefaultProjectName(""))
}

func TestProjectMatchesDefault(t *testing.T) {
	t.Parallel()

	def := &PCDefaultProject{UUID: "def-uuid", Name: "_internal"}
	assert.True(t, ProjectMatchesDefault(&capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new("def-uuid"),
	}, def))
	assert.True(t, ProjectMatchesDefault(&capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierName,
		Name: new("_internal"),
	}, def))
	assert.False(t, ProjectMatchesDefault(&capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new("other"),
	}, def))
}

func TestIsAuthorizationError(t *testing.T) {
	t.Parallel()
	assert.True(t, isAuthorizationError(errors.New("403 Forbidden")))
	assert.True(t, isAuthorizationError(errors.New("permission denied")))
	assert.False(t, isAuthorizationError(errors.New("resource not found")))
}
