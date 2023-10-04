// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package customimage

import (
	"testing"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/docker/mutation/customimage/tests"
)

func TestControlPlaneGeneratePatches(t *testing.T) {
	t.Parallel()

	tests.TestControlPlaneGeneratePatches(
		t,
		func() mutation.GeneratePatches { return NewControlPlanePatch() },
		VariableName,
	)
}
