// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/textlogger"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var TestEnv *TestEnvironment

// Initialize the test environment. BeforeSuite will be only executed if this package is loaded by the test.
var _ = BeforeSuite(func(ctx SpecContext) {
	By("Initialize loggers for testing")
	// Uninitialized logger spits stacktrace as warning during test execution
	logger := textlogger.NewLogger(textlogger.NewConfig())
	// use klog as the internal logger for this envtest environment.
	log.SetLogger(logger)
	// additionally force all of the controllers to use the Ginkgo logger.
	ctrl.SetLogger(logger)
	klog.InitFlags(nil)
	// add logger for ginkgo
	klog.SetOutput(GinkgoWriter)

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
