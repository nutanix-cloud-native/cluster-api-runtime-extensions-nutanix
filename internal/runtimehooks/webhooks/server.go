// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package webhooks

import (
	"context"
	"fmt"

	"github.com/spf13/pflag"
	runtimecatalog "sigs.k8s.io/cluster-api/exp/runtime/catalog"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/server"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/lifecycle"
)

type Server struct {
	webhookPort    int
	webhookCertDir string
	addonProvider  lifecycle.AddonProvider

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
		addonProvider:  lifecycle.ClusterResourceSetAddonProvider,
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

	fs.Var(newAddonProviderValue(
		lifecycle.ClusterResourceSetAddonProvider, &s.addonProvider),
		prefix+".addon-provider",
		fmt.Sprintf(
			"addon provider (one of %v)",
			[]string{
				string(lifecycle.ClusterResourceSetAddonProvider),
				string(lifecycle.FluxHelmReleaseAddonProvider),
			},
		),
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

	// Create the ExtensionHandlers for the lifecycle hooks
	lifecycleExtensionHandlers := lifecycle.NewExtensionHandlers(s.addonProvider, client)

	// Register extension handlers.
	if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
		Hook:        runtimehooksv1.BeforeClusterCreate,
		Name:        "before-cluster-create",
		HandlerFunc: lifecycleExtensionHandlers.DoBeforeClusterCreate,
	}); err != nil {
		setupLog.Error(err, "error adding handler")
		return err
	}
	if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
		Hook:        runtimehooksv1.AfterControlPlaneInitialized,
		Name:        "after-control-plane-initialized",
		HandlerFunc: lifecycleExtensionHandlers.DoAfterControlPlaneInitialized,
	}); err != nil {
		setupLog.Error(err, "error adding handler")
		return err
	}
	if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
		Hook:        runtimehooksv1.BeforeClusterUpgrade,
		Name:        "before-cluster-upgrade",
		HandlerFunc: lifecycleExtensionHandlers.DoBeforeClusterUpgrade,
	}); err != nil {
		setupLog.Error(err, "error adding handler")
		return err
	}
	if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
		Hook:        runtimehooksv1.BeforeClusterDelete,
		Name:        "before-cluster-delete",
		HandlerFunc: lifecycleExtensionHandlers.DoBeforeClusterDelete,
	}); err != nil {
		setupLog.Error(err, "error adding handler")
		return err
	}

	// Start the https server.
	setupLog.Info("Starting Runtime Extension server")
	if err := webhookServer.Start(ctx); err != nil {
		setupLog.Error(err, "error running webhook server")
		return err
	}

	return nil
}
