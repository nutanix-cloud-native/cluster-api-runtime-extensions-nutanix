// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package webhooks

import (
	"context"
	"strings"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/strings/slices"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	crsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	runtimecatalog "sigs.k8s.io/cluster-api/exp/runtime/catalog"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/server"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/cni/calico"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/httpproxy"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/servicelbgc"
)

type Server struct {
	webhookPort    int
	webhookCertDir string

	catalog *runtimecatalog.Catalog

	enabledHandlers []string

	calicoCNIConfig *calico.CalicoCNIConfig
}

func NewServer() *Server {
	// catalog contains all information about RuntimeHooks.
	catalog := runtimecatalog.New()

	_ = runtimehooksv1.AddToCatalog(catalog)

	return &Server{
		catalog:         catalog,
		webhookPort:     9443,
		webhookCertDir:  "/runtimehooks-certs/",
		calicoCNIConfig: &calico.CalicoCNIConfig{},
	}
}

func (s *Server) AddFlags(prefix string, fs *pflag.FlagSet) {
	fs.IntVar(&s.webhookPort, prefix+".port", s.webhookPort, "Webhook Server port")

	fs.StringVar(
		&s.webhookCertDir,
		prefix+".cert-dir",
		s.webhookCertDir,
		"Runtime hooks server cert dir.",
	)

	fs.StringSliceVar(
		&s.enabledHandlers,
		prefix+".enabled-handlers",
		[]string{"ServiceLoadBalancerGC", "CalicoCNI", "http-proxy-patch", "http-proxy-vars"},
		"list of all enabled handlers",
	)

	s.calicoCNIConfig.AddFlags(prefix+".calicocni", fs)
}

func (s *Server) Start(ctx context.Context) error {
	// Creates a logger to be used during the main func.
	setupLog := ctrl.Log.WithName("runtimehooks")

	// Create a http server for serving runtime extensions
	webhookServer, err := server.New(server.Options{
		Catalog: s.catalog,
		Port:    s.webhookPort,
		CertDir: s.webhookCertDir,
	})
	if err != nil {
		setupLog.Error(err, "error creating webhook server")
		return err
	}

	// Lifecycle Hooks

	// Gets a client to access the Kubernetes cluster where this RuntimeExtension will be deployed to
	restConfig, err := ctrl.GetConfig()
	if err != nil {
		setupLog.Error(err, "error getting config for the cluster")
		return err
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(crsv1.AddToScheme(scheme))
	utilruntime.Must(capiv1.AddToScheme(scheme))

	client, err := ctrclient.New(restConfig, ctrclient.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "error creating client to the cluster")
		return err
	}

	allHandlers := []handlers.NamedHandler{
		servicelbgc.New(client),
		calico.New(client, *s.calicoCNIConfig),
		httpproxy.NewVariable(),
		httpproxy.NewPatch(),
	}

	for idx := range allHandlers {
		h := allHandlers[idx]

		if !slices.Contains(s.enabledHandlers, h.Name()) {
			continue
		}

		if t, ok := h.(handlers.BeforeClusterCreateLifecycleHandler); ok {
			if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
				Hook:        runtimehooksv1.BeforeClusterCreate,
				Name:        strings.ToLower(h.Name()),
				HandlerFunc: t.BeforeClusterCreate,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(handlers.AfterControlPlaneInitializedLifecycleHandler); ok {
			if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
				Hook:        runtimehooksv1.AfterControlPlaneInitialized,
				Name:        strings.ToLower(h.Name()),
				HandlerFunc: t.AfterControlPlaneInitialized,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(handlers.BeforeClusterUpgradeLifecycleHandler); ok {
			if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
				Hook:        runtimehooksv1.BeforeClusterUpgrade,
				Name:        strings.ToLower(h.Name()),
				HandlerFunc: t.BeforeClusterUpgrade,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(handlers.AfterControlPlaneUpgradeLifecycleHandler); ok {
			if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
				Hook:        runtimehooksv1.AfterControlPlaneUpgrade,
				Name:        h.Name(),
				HandlerFunc: t.AfterControlPlaneUpgrade,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(handlers.BeforeClusterDeleteLifecycleHandler); ok {
			if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
				Hook:        runtimehooksv1.BeforeClusterDelete,
				Name:        strings.ToLower(h.Name()),
				HandlerFunc: t.BeforeClusterDelete,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(handlers.DiscoverVariablesMutationHandler); ok {
			if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
				Hook:        runtimehooksv1.DiscoverVariables,
				Name:        strings.ToLower(h.Name()),
				HandlerFunc: t.DiscoverVariables,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(handlers.GeneratePatchesMutationHandler); ok {
			if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
				Hook:        runtimehooksv1.GeneratePatches,
				Name:        strings.ToLower(h.Name()),
				HandlerFunc: t.GeneratePatches,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(handlers.ValidateTopologyMutationHandler); ok {
			if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
				Hook:        runtimehooksv1.ValidateTopology,
				Name:        strings.ToLower(h.Name()),
				HandlerFunc: t.ValidateTopology,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}
	}

	// Start the https server.
	setupLog.Info("Starting Runtime Extension server")
	if err := webhookServer.Start(ctx); err != nil {
		setupLog.Error(err, "error running webhook server")
		return err
	}

	return nil
}
