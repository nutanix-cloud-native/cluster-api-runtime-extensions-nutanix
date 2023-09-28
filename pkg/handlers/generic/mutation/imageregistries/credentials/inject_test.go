// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/imageregistries/credentials/tests"
)

func TestGeneratePatches(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()

	tests.TestGeneratePatches(
		t,
		func() mutation.GeneratePatches {
			return NewPatch(fakeClient)
		},
		fakeClient,
		VariableName,
	)
}
