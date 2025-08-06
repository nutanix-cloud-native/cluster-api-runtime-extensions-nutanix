// Copyright 2023 Nutanix. All rights reserved.
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
	crsv1 "sigs.k8s.io/cluster-api/api/addons/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/server"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/controllers/enforceclusterautoscalerlimits"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/controllers/failuredomainrollout"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/controllers/namespacesync"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/feature"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/eks"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/cluster"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
	preflightgeneric "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight/generic"
	preflightnutanix "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight/nutanix"
)

func main() {
	// Creates a logger to be used during the main func.
	setupLog := ctrl.Log.WithName("main")

	clientScheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
	utilruntime.Must(crsv1.AddToScheme(clientScheme))
	utilruntime.Must(clusterv1.AddToScheme(clientScheme))
	utilruntime.Must(controlplanev1.AddToScheme(clientScheme))
	utilruntime.Must(caaphv1.AddToScheme(clientScheme))
	utilruntime.Must(capxv1.AddToScheme(clientScheme))

	webhookOptions := webhook.Options{
		Port:    9444,
		CertDir: "/admission-certs",
	}
	mgrOptions := &ctrl.Options{
		Scheme: clientScheme,
		Metrics: metricsserver.Options{
			BindAddress: ":8080",
		},
		HealthProbeBindAddress: ":8081",
		LeaderElection:         false,
		WebhookServer:          webhook.NewServer(webhookOptions),
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

	pflag.CommandLine.StringVar(
		&webhookOptions.CertDir,
		"admission-webhook-cert-dir",
		webhookOptions.CertDir,
		"Admission webhooks server cert dir.",
	)

	logOptions := logs.NewOptions()

	runtimeWebhookServerOpts := server.NewServerOptions()

	globalOptions := options.NewGlobalOptions()

	genericLifecycleHandlers := lifecycle.New(globalOptions)

	// awsMetaHandlers combines all AWS patch and variable handlers under a single handler.
	// It allows to specify configuration under a single variable.
	awsMetaHandlers := aws.New(globalOptions)

	// dockerMetaHandlers combines all Docker patch and variable handlers under a single handler.
	// It allows to specify configuration under a single variable.
	dockerMetaHandlers := docker.New(globalOptions)

	// nutanixMetaHandlers combines all Nutanix patch and variable handlers under a single handler.
	// It allows to specify configuration under a single variable.
	nutanixMetaHandlers := nutanix.New(globalOptions)

	// genericMetaHandlers combines all generic patch and variable handlers under a single handler.
	// It allows to specify configuration under a single variable.
	genericMetaHandlers := generic.New()

	// eksMetaHandlers combines all EKS patch and variable handlers under a single handler.
	// It allows to specify configuration under a single variable.
	eksMetaHandlers := eks.New(globalOptions)

	namespacesyncOptions := namespacesync.Options{}
	enforceClusterAutoscalerLimitsOptions := enforceclusterautoscalerlimits.Options{}
	failureDomainRolloutOptions := failuredomainrollout.Options{}

	// Initialize and parse command line flags.
	logs.AddFlags(pflag.CommandLine, logs.SkipLoggingConfigurationFlags())
	logsv1.AddFlags(logOptions, pflag.CommandLine)
	globalOptions.AddFlags(pflag.CommandLine)
	runtimeWebhookServerOpts.AddFlags(pflag.CommandLine)
	genericLifecycleHandlers.AddFlags(pflag.CommandLine)
	awsMetaHandlers.AddFlags(pflag.CommandLine)
	dockerMetaHandlers.AddFlags(pflag.CommandLine)
	nutanixMetaHandlers.AddFlags(pflag.CommandLine)
	namespacesyncOptions.AddFlags(pflag.CommandLine)
	enforceClusterAutoscalerLimitsOptions.AddFlags(pflag.CommandLine)
	failureDomainRolloutOptions.AddFlags(pflag.CommandLine)
	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	// Add feature gate flag.
	feature.MutableGates.AddFlag(pflag.CommandLine)

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

	var allHandlers []handlers.Named
	allHandlers = append(allHandlers, genericLifecycleHandlers.AllHandlers(mgr)...)
	allHandlers = append(allHandlers, awsMetaHandlers.AllHandlers(mgr)...)
	allHandlers = append(allHandlers, dockerMetaHandlers.AllHandlers(mgr)...)
	allHandlers = append(allHandlers, nutanixMetaHandlers.AllHandlers(mgr)...)
	allHandlers = append(allHandlers, genericMetaHandlers.AllHandlers(mgr)...)
	allHandlers = append(allHandlers, eksMetaHandlers.AllHandlers(mgr)...)

	runtimeWebhookServer := server.NewServer(runtimeWebhookServerOpts, allHandlers...)

	if err := mgr.Add(runtimeWebhookServer); err != nil {
		setupLog.Error(err, "unable to add runtime webhook server runnable to controller manager")
		os.Exit(1)
	}

	if namespacesyncOptions.Enabled {
		if namespacesyncOptions.SourceNamespace == "" ||
			namespacesyncOptions.TargetNamespaceLabelKey == "" {
			setupLog.Error(
				nil,
				"Namespace Sync is enabled, but source namespace and/or target namespace label key are not configured.",
			)
			os.Exit(1)
		}

		unstructuredCachingClient, err := client.New(mgr.GetConfig(), client.Options{
			HTTPClient: mgr.GetHTTPClient(),
			Cache: &client.CacheOptions{
				Reader:       mgr.GetCache(),
				Unstructured: true,
			},
		})
		if err != nil {
			setupLog.Error(err, "unable to create unstructured caching client")
			os.Exit(1)
		}

		if err := (&namespacesync.Reconciler{
			Client:                      mgr.GetClient(),
			UnstructuredCachingClient:   unstructuredCachingClient,
			SourceClusterClassNamespace: namespacesyncOptions.SourceNamespace,
			IsTargetNamespace:           namespacesync.NamespaceHasLabelKey(namespacesyncOptions.TargetNamespaceLabelKey),
		}).SetupWithManager(
			mgr,
			&controller.Options{MaxConcurrentReconciles: namespacesyncOptions.Concurrency},
		); err != nil {
			setupLog.Error(
				err,
				"unable to create controller",
				"controller",
				"namespacesync.Reconciler",
			)
			os.Exit(1)
		}
	}

	if enforceClusterAutoscalerLimitsOptions.Enabled {
		if err := (&enforceclusterautoscalerlimits.Reconciler{
			Client: mgr.GetClient(),
		}).SetupWithManager(
			mgr,
			&controller.Options{MaxConcurrentReconciles: enforceClusterAutoscalerLimitsOptions.Concurrency},
		); err != nil {
			setupLog.Error(
				err,
				"unable to create controller",
				"controller",
				"enforceclusterautoscalerlimits.Reconciler",
			)
			os.Exit(1)
		}
	}

	if failureDomainRolloutOptions.Enabled {
		if err := (&failuredomainrollout.Reconciler{
			Client: mgr.GetClient(),
		}).SetupWithManager(
			mgr,
			&controller.Options{MaxConcurrentReconciles: failureDomainRolloutOptions.Concurrency},
		); err != nil {
			setupLog.Error(
				err,
				"unable to create controller",
				"controller",
				"failuredomainrollout.Reconciler",
			)
			os.Exit(1)
		}
	}

	mgr.GetWebhookServer().Register("/mutate-v1beta1-cluster", &webhook.Admission{
		Handler: cluster.NewDefaulter(mgr.GetClient(), admission.NewDecoder(mgr.GetScheme())),
	})
	mgr.GetWebhookServer().Register("/validate-v1beta1-cluster", &webhook.Admission{
		Handler: cluster.NewValidator(mgr.GetClient(), admission.NewDecoder(mgr.GetScheme())),
	})

	mgr.GetWebhookServer().Register("/mutate-v1beta1-addons", &webhook.Admission{
		Handler: addons.NewDefaulter(mgr.GetClient(), admission.NewDecoder(mgr.GetScheme())),
	})

	mgr.GetWebhookServer().Register("/preflight-v1beta1-cluster", &webhook.Admission{
		Handler: preflight.New(mgr.GetClient(), admission.NewDecoder(mgr.GetScheme()),
			[]preflight.Checker{
				// Add your preflight checkers here.
				preflightgeneric.Checker,
				preflightnutanix.Checker,
			}...,
		),
	})
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
