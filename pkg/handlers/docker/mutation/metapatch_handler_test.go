// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"testing"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/docker/mutation/customimage"
	customimagetests "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/docker/mutation/customimage/tests"
)

func metaPatchGeneratorFunc(mgr manager.Manager) func() mutation.GeneratePatches {
	return func() mutation.GeneratePatches {
		return MetaPatchHandler(mgr).(mutation.GeneratePatches)
	}
}

func TestGeneratePatches(t *testing.T) {
	t.Parallel()

	mgr, _ := manager.New(
		&rest.Config{},
		manager.Options{
			NewClient: func(_ *rest.Config, _ client.Options) (client.Client, error) {
				return fake.NewClientBuilder().Build(), nil
			},
		},
	)

	customimagetests.TestGeneratePatches(
		t,
		metaPatchGeneratorFunc(mgr),
		"clusterConfig",
		customimage.VariableName,
	)
}
