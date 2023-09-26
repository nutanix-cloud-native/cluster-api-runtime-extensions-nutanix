// Copyright 2023 D2iQ, Inc. All rights reserved.
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

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/server"
	awsclusterconfig "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/aws/clusterconfig"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/aws/mutation/region"
	dockerclusterconfig "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/docker/clusterconfig"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/docker/mutation/customimage"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/lifecycle/cni/calico"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/lifecycle/nfd"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/lifecycle/servicelbgc"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/auditpolicy"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/etcd"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/extraapiservercertsans"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/httpproxy"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/kubernetesimagerepository"
)

// Flags.
var logOptions = logs.NewOptions()

// initFlags initializes the flags.
func initFlags(fs *pflag.FlagSet) {
	// Initialize logs flags using Kubernetes component-base machinery.
	logs.AddFlags(fs, logs.SkipLoggingConfigurationFlags())
	logsv1.AddFlags(logOptions, fs)
}

func main() {
	// Creates a logger to be used during the main func.
	setupLog := ctrl.Log.WithName("main")

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(crsv1.AddToScheme(scheme))
	utilruntime.Must(capiv1.AddToScheme(scheme))

	mgrOptions := &ctrl.Options{
		Scheme: scheme,
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

	calicoCNIConfig := &calico.CalicoCNIConfig{}
	nfdConfig := &nfd.NFDConfig{}

	runtimeWebhookServerOpts := server.NewServerOptions()

	// Initialize and parse command line flags.
	initFlags(pflag.CommandLine)
	runtimeWebhookServerOpts.AddFlags(pflag.CommandLine)
	nfdConfig.AddFlags("nfd", pflag.CommandLine)
	calicoCNIConfig.AddFlags("calicocni", pflag.CommandLine)
	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	verflag.PrintAndExitIfRequested()

	// Validates logs flags using Kubernetes component-base machinery and applies them
	if err := logsv1.ValidateAndApply(logOptions, nil); err != nil {
		setupLog.Error(err, "unable to start extension")
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

	// Handlers for lifecycle hooks.
	genericLifecycleHandlers := []handlers.Named{
		calico.New(mgr.GetClient(), calicoCNIConfig),
		nfd.New(mgr.GetClient(), nfdConfig),
		servicelbgc.New(mgr.GetClient()),
	}

	// This genericMetaPatchHandlers combines all other patch and variable handlers under a single handler.
	// It allows to specify configuration under a single variable.
	genericMetaPatchHandlers := []mutation.MetaMutater{
		auditpolicy.NewPatch(),
		etcd.NewMetaPatch(),
		extraapiservercertsans.NewMetaPatch(),
		httpproxy.NewMetaPatch(mgr.GetClient()),
		kubernetesimagerepository.NewMetaPatch(),
	}

	// awsMetaPatchHandlers combines all AWS patch and variable handlers under a single handler.
	// It allows to specify configuration under a single variable.
	awsMetaPatchHandlers := append(
		[]mutation.MetaMutater{
			region.NewMetaPatch(),
		},
		genericMetaPatchHandlers...,
	)
	awsMetaHandlers := []handlers.Named{
		awsclusterconfig.NewVariable(),
		mutation.NewMetaGeneratePatchesHandler("awsClusterConfigPatch", awsMetaPatchHandlers...),
	}

	// dockerMetaPatchHandlers combines all Docker patch and variable handlers under a single handler.
	// It allows to specify configuration under a single variable.
	dockerMetaPatchHandlers := append(
		[]mutation.MetaMutater{
			customimage.NewMetaPatch(),
		},
		genericMetaPatchHandlers...,
	)
	dockerMetaHandlers := []handlers.Named{
		dockerclusterconfig.NewVariable(),
		mutation.NewMetaGeneratePatchesHandler(
			"dockerClusterConfigPatch",
			dockerMetaPatchHandlers...),
	}

	var allHandlers []handlers.Named
	allHandlers = append(allHandlers, genericLifecycleHandlers...)
	allHandlers = append(allHandlers, awsMetaHandlers...)
	allHandlers = append(allHandlers, dockerMetaHandlers...)

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
