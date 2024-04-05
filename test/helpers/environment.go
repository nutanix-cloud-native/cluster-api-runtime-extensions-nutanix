// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var TestEnv *TestEnvironment

// Initialize the test environment. BeforeSuite will be only executed if this package is loaded by the test.
var _ = BeforeSuite(func(ctx SpecContext) {
	By("Starting test environment")
	testEnvConfig := NewTestEnvironmentConfiguration()
	var err error
	TestEnv, err = testEnvConfig.Build()
	if err != nil {
		panic(err)
	}
	By("Starting the manager")
	go func() {
		defer GinkgoRecover()
		Expect(TestEnv.StartManager(ctx)).To(Succeed())
	}()
}, NodeTimeout(60*time.Second))

var _ = AfterSuite(func(ctx context.Context) {
	Expect(TestEnv.Stop()).To(Succeed())
})
