// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"context"
	"fmt"
	"testing"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var (
	testEnv *helpers.TestEnvironment
	ctx     = ctrl.SetupSignalHandler()
)

func TestMain(m *testing.M) {
	setupCtx, cancel := context.WithCancel(ctx)
	setup(setupCtx)
	defer teardown(cancel)
	m.Run()
}

func setup(ctx context.Context) {
	testEnvConfig := helpers.NewTestEnvironmentConfiguration()
	var err error
	testEnv, err = testEnvConfig.Build()
	if err != nil {
		panic(err)
	}
	go func() {
		fmt.Println("Starting the manager")
		if err := testEnv.StartManager(ctx); err != nil {
			panic(fmt.Sprintf("Failed to start the envtest manager: %v", err))
		}
	}()
}

func teardown(cancel context.CancelFunc) {
	cancel()
	if err := testEnv.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop envtest: %v", err))
	}
}
