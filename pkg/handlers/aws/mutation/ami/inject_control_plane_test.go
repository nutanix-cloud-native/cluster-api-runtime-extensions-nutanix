// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ami

import (
	"testing"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	awsclusterconfig "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/aws/clusterconfig"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/aws/mutation/ami/tests"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
)

func TestControlPlaneAMIGeneratePatches(t *testing.T) {
	t.Parallel()

	tests.TestControlPlaneGeneratePatches(
		t,
		func() mutation.GeneratePatches { return NewControlPlanePatch() },
		clusterconfig.MetaVariableName,
		clusterconfig.MetaControlPlaneConfigName,
		awsclusterconfig.AWSVariableName,
		VariableName,
	)
}
