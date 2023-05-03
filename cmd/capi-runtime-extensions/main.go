// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/d2iq-labs/capi-runtime-extensions/internal/controllermanager"
	runtimewebhooks "github.com/d2iq-labs/capi-runtime-extensions/internal/runtimehooks/webhooks"
)

var (
	// Flags.
	profilerAddress string
	logOptions      = logs.NewOptions()
)

// initFlags initializes the flags.
func initFlags(fs *pflag.FlagSet) {
	// Initialize logs flags using Kubernetes component-base machinery.
	logs.AddFlags(fs, logs.SkipLoggingConfigurationFlags())
	logsv1.AddFlags(logOptions, fs)

	// Add test-extension specific flags
	fs.StringVar(&profilerAddress, "profiler-address", "",
		"Bind address to expose the pprof profiler (e.g. localhost:6060)")
}

func main() {
	// Creates a logger to be used during the main func.
	setupLog := ctrl.Log.WithName("main")

	runtimeWebhookServer := runtimewebhooks.NewServer()
	controllers := controllermanager.New()

	// Initialize and parse command line flags.
	initFlags(pflag.CommandLine)
	runtimeWebhookServer.AddFlags("runtimehooks", pflag.CommandLine)
	controllers.AddFlags("controllermanager", pflag.CommandLine)
	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	// Validates logs flags using Kubernetes component-base machinery and applies them
	if err := logsv1.ValidateAndApply(logOptions, nil); err != nil {
		setupLog.Error(err, "unable to start extension")
		os.Exit(1)
	}

	// Add the klog logger in the context.
	ctrl.SetLogger(klog.Background())

	// Initialize the golang profiler server, if required.
	if profilerAddress != "" {
		klog.Infof("Profiler listening for requests at %s", profilerAddress)
		go func() {
			profilerServer := &http.Server{
				Addr:              profilerAddress,
				Handler:           nil,
				MaxHeaderBytes:    1 << 20,
				IdleTimeout:       90 * time.Second, // matches http.DefaultTransport keep-alive timeout
				ReadHeaderTimeout: 32 * time.Second,
			}
			klog.Info(profilerServer.ListenAndServe())
		}()
	}

	signalCtx := ctrl.SetupSignalHandler()
	g, ctx := errgroup.WithContext(signalCtx)

	g.Go(func() error {
		err := runtimeWebhookServer.Start(ctx)
		if err != nil {
			setupLog.Error(err, "unable to start runtime hooks wehook server")
		}
		return err
	})

	g.Go(func() error {
		err := controllers.Start(ctx)
		if err != nil {
			setupLog.Error(err, "unable to start controller manager")
		}
		return err
	})

	if err := g.Wait(); err != nil {
		setupLog.Error(err, "failed to run successfully")
		os.Exit(1)
	}
}
