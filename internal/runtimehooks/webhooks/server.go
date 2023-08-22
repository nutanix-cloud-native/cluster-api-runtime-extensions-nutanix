// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package webhooks

import (
	"context"

	"github.com/spf13/pflag"
	runtimecatalog "sigs.k8s.io/cluster-api/exp/runtime/catalog"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/server"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/servicelbgc"
)

type Server struct {
	webhookPort    int
	webhookCertDir string

	catalog *runtimecatalog.Catalog
}

func NewServer() *Server {
	// catalog contains all information about RuntimeHooks.
	catalog := runtimecatalog.New()

	_ = runtimehooksv1.AddToCatalog(catalog)

	return &Server{
		catalog:        catalog,
		webhookPort:    9443,
		webhookCertDir: "/runtimehooks-certs/",
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

	client, err := ctrclient.New(restConfig, ctrclient.Options{})
	if err != nil {
		setupLog.Error(err, "error creating client to the cluster")
		return err
	}

	allHandlers := []handlers.NamedHandler{servicelbgc.New(client)}

	for idx := range allHandlers {
		h := allHandlers[idx]

		if t, ok := h.(handlers.BeforeClusterCreateLifecycleHandler); ok {
			if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
				Hook:        runtimehooksv1.BeforeClusterCreate,
				Name:        h.Name(),
				HandlerFunc: t.BeforeClusterCreate,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(handlers.AfterControlPlaneInitializedLifecycleHandler); ok {
			if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
				Hook:        runtimehooksv1.AfterControlPlaneInitialized,
				Name:        h.Name(),
				HandlerFunc: t.AfterControlPlaneInitialized,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(handlers.BeforeClusterUpgradeLifecycleHandler); ok {
			if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
				Hook:        runtimehooksv1.BeforeClusterUpgrade,
				Name:        h.Name(),
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
				Name:        h.Name(),
				HandlerFunc: t.BeforeClusterDelete,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(handlers.DiscoverVariablesMutationHandler); ok {
			if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
				Hook:        runtimehooksv1.DiscoverVariables,
				Name:        h.Name(),
				HandlerFunc: t.DiscoverVariables,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(handlers.GeneratePatchesMutationHandler); ok {
			if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
				Hook:        runtimehooksv1.GeneratePatches,
				Name:        h.Name(),
				HandlerFunc: t.GeneratePatches,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(handlers.ValidateTopologyMutationHandler); ok {
			if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
				Hook:        runtimehooksv1.ValidateTopology,
				Name:        h.Name(),
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
