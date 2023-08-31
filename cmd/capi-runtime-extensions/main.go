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
	ctrclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/internal/controllermanager"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/apiservercertsans"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/cni/calico"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/httpproxy"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/servicelbgc"
	"github.com/d2iq-labs/capi-runtime-extensions/server/pkg/server"
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

	controllers := controllermanager.New()

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(crsv1.AddToScheme(scheme))
	utilruntime.Must(capiv1.AddToScheme(scheme))

	// Gets a client to access the Kubernetes cluster where this RuntimeExtension will be deployed to
	restConfig, err := ctrl.GetConfig()
	if err != nil {
		setupLog.Error(err, "error getting config for the cluster")
		os.Exit(1)
	}

	client, err := ctrclient.New(restConfig, ctrclient.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "error creating client to the cluster")
		os.Exit(1)
	}

	calicoCNIConfig := &calico.CalicoCNIConfig{}

	runtimeWebhookServer := server.NewServer(
		servicelbgc.New(client),
		calico.New(client, calicoCNIConfig),
		httpproxy.NewVariable(),
		httpproxy.NewPatch(),
		apiservercertsans.NewVariable(),
		apiservercertsans.NewPatch(),
	)

	// Initialize and parse command line flags.
	initFlags(pflag.CommandLine)
	runtimeWebhookServer.AddFlags("runtimehooks", pflag.CommandLine)
	controllers.AddFlags("controllermanager", pflag.CommandLine)
	calicoCNIConfig.AddFlags("runtimehooks.calicocni", pflag.CommandLine)
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
