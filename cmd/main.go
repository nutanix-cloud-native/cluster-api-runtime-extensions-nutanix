// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	"k8s.io/component-base/version/verflag"
	"k8s.io/klog/v2"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	crsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	caaphv1 "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/server"
	awsclusterconfig "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/clusterconfig"
	awsmutation "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation"
	awsworkerconfig "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/workerconfig"
	dockerclusterconfig "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/clusterconfig"
	dockermutation "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/mutation"
	dockerworkerconfig "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/workerconfig"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle"
	nutanixclusterconfig "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
	nutanixmutation "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/mutation"
	nutanixworkerconfig "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/workerconfig"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

func main() {
	// Creates a logger to be used during the main func.
	setupLog := ctrl.Log.WithName("main")

	clientScheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
	utilruntime.Must(crsv1.AddToScheme(clientScheme))
	utilruntime.Must(capiv1.AddToScheme(clientScheme))
	utilruntime.Must(caaphv1.AddToScheme(clientScheme))

	mgrOptions := &ctrl.Options{
		Scheme: clientScheme,
		Metrics: metricsserver.Options{
			BindAddress: ":8080",
		},
		HealthProbeBindAddress: ":8081",
		LeaderElection:         false,
	}

	pflag.CommandLine.StringVar(
		&mgrOptions.Metrics.BindAddress,
		"metrics-bind-address",
		mgrOptions.Metrics.BindAddress,
		"The address the metric endpoint binds to.",
	)

	pflag.CommandLine.StringVar(
		&mgrOptions.HealthProbeBindAddress,
		"health-probe-bind-address",
		mgrOptions.HealthProbeBindAddress,
		"The address the probe endpoint binds to.",
	)

	pflag.CommandLine.StringVar(&mgrOptions.PprofBindAddress, "profiler-address", "",
		"Bind address to expose the pprof profiler (e.g. localhost:6060)")

	logOptions := logs.NewOptions()

	runtimeWebhookServerOpts := server.NewServerOptions()

	globalOptions := options.NewGlobalOptions()

	genericLifecycleHandlers := lifecycle.New(globalOptions)

	// Initialize and parse command line flags.
	logs.AddFlags(pflag.CommandLine, logs.SkipLoggingConfigurationFlags())
	logsv1.AddFlags(logOptions, pflag.CommandLine)
	globalOptions.AddFlags(pflag.CommandLine)
	runtimeWebhookServerOpts.AddFlags(pflag.CommandLine)
	genericLifecycleHandlers.AddFlags(pflag.CommandLine)
	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	verflag.PrintAndExitIfRequested()

	// Validates logs flags using Kubernetes component-base machinery and applies them
	if err := logsv1.ValidateAndApply(logOptions, nil); err != nil {
		setupLog.Error(err, "unable to apply logging configuration")
		os.Exit(1)
	}

	// Add the klog logger in the context.
	ctrl.SetLogger(klog.Background())

	signalCtx := ctrl.SetupSignalHandler()

	mgr, err := newManager(mgrOptions)
	if err != nil {
		setupLog.Error(err, "failed to create a new controller manager")
		os.Exit(1)
	}

	// This genericMetaPatchHandlers combines all other patch and variable handlers under a single handler.
	// It allows to specify configuration under a single variable.
	// awsMetaHandlers combines all AWS patch and variable handlers under a single handler.
	awsMetaHandlers := []handlers.Named{
		awsclusterconfig.NewVariable(),
		awsworkerconfig.NewVariable(),
		awsmutation.MetaPatchHandler(mgr),
		awsmutation.MetaWorkerPatchHandler(),
	}

	// dockerMetaHandlers combines all Docker patch and variable handlers under a single handler.
	// It allows to specify configuration under a single variable.
	dockerMetaHandlers := []handlers.Named{
		dockerclusterconfig.NewVariable(),
		dockerworkerconfig.NewVariable(),
		dockermutation.MetaPatchHandler(mgr),
		dockermutation.MetaWorkerPatchHandler(),
	}

	// nutanixMetaHandlers combines all Nutanix patch and variable handlers under a single handler.
	// It allows to specify configuration under a single variable.
	nutanixMetaHandlers := []handlers.Named{
		nutanixclusterconfig.NewVariable(),
		nutanixworkerconfig.NewVariable(),
		nutanixmutation.MetaPatchHandler(mgr),
		nutanixmutation.MetaWorkerPatchHandler(),
	}

	var allHandlers []handlers.Named
	allHandlers = append(allHandlers, genericLifecycleHandlers.AllHandlers(mgr)...)
	allHandlers = append(allHandlers, awsMetaHandlers...)
	allHandlers = append(allHandlers, dockerMetaHandlers...)
	allHandlers = append(allHandlers, nutanixMetaHandlers...)

	runtimeWebhookServer := server.NewServer(runtimeWebhookServerOpts, allHandlers...)

	if err := mgr.Add(runtimeWebhookServer); err != nil {
		setupLog.Error(err, "unable to add runtime webhook server runnable to controller manager")
		os.Exit(1)
	}

	if err := mgr.Start(signalCtx); err != nil {
		setupLog.Error(err, "unable to start controller manager")
		os.Exit(1)
	}
}

func newManager(opts *manager.Options) (ctrl.Manager, error) {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), *opts)
	if err != nil {
		return nil, fmt.Errorf("unable to create manager: %w", err)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return nil, fmt.Errorf("unable to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return nil, fmt.Errorf("unable to set up ready check: %w", err)
	}

	return mgr, nil
}
