// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"strings"

	"github.com/spf13/pflag"
	runtimecatalog "sigs.k8s.io/cluster-api/exp/runtime/catalog"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	runtimeserver "sigs.k8s.io/cluster-api/exp/runtime/server"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
)

type Server struct {
	catalog *runtimecatalog.Catalog
	hooks   []handlers.Named

	opts *ServerOptions
}

func NewServer(opts *ServerOptions, hooks ...handlers.Named) *Server {
	// catalog contains all information about RuntimeHooks.
	catalog := runtimecatalog.New()

	_ = runtimehooksv1.AddToCatalog(catalog)

	return &Server{
		catalog: catalog,
		opts:    opts,
		hooks:   hooks,
	}
}

type ServerOptions struct {
	webhookPort    int
	webhookCertDir string
}

func NewServerOptions() *ServerOptions {
	return &ServerOptions{}
}

func (s *ServerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.IntVar(&s.webhookPort, "webhook-port", s.webhookPort, "Webhook Server port")

	fs.StringVar(
		&s.webhookCertDir,
		"webhook-cert-dir",
		s.webhookCertDir,
		"Runtime hooks server cert dir.",
	)
}

// NeedLeaderElection implements the LeaderElectionRunnable interface, which indicates
// the webhook server doesn't need leader election.
func (*Server) NeedLeaderElection() bool {
	return false
}

func (s *Server) Start(ctx context.Context) error {
	// Creates a logger to be used during the main func.
	setupLog := ctrl.Log.WithName("runtimehooks")

	// Create a http server for serving runtime extensions
	webhookServer, err := runtimeserver.New(runtimeserver.Options{
		Catalog: s.catalog,
		Port:    s.opts.webhookPort,
		CertDir: s.opts.webhookCertDir,
	})
	if err != nil {
		setupLog.Error(err, "error creating webhook server")
		return err
	}

	for _, h := range s.hooks {
		if t, ok := h.(lifecycle.BeforeClusterCreate); ok {
			if err := webhookServer.AddExtensionHandler(runtimeserver.ExtensionHandler{
				Hook:        runtimehooksv1.BeforeClusterCreate,
				Name:        strings.ToLower(h.Name()) + "-bcc",
				HandlerFunc: t.BeforeClusterCreate,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(lifecycle.AfterControlPlaneInitialized); ok {
			if err := webhookServer.AddExtensionHandler(runtimeserver.ExtensionHandler{
				Hook:        runtimehooksv1.AfterControlPlaneInitialized,
				Name:        strings.ToLower(h.Name()) + "-acpi",
				HandlerFunc: t.AfterControlPlaneInitialized,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(lifecycle.BeforeClusterUpgrade); ok {
			if err := webhookServer.AddExtensionHandler(runtimeserver.ExtensionHandler{
				Hook:        runtimehooksv1.BeforeClusterUpgrade,
				Name:        strings.ToLower(h.Name()) + "-bcu",
				HandlerFunc: t.BeforeClusterUpgrade,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(lifecycle.AfterControlPlaneUpgrade); ok {
			if err := webhookServer.AddExtensionHandler(runtimeserver.ExtensionHandler{
				Hook:        runtimehooksv1.AfterControlPlaneUpgrade,
				Name:        h.Name() + "-acpu",
				HandlerFunc: t.AfterControlPlaneUpgrade,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(lifecycle.BeforeClusterDelete); ok {
			if err := webhookServer.AddExtensionHandler(runtimeserver.ExtensionHandler{
				Hook:        runtimehooksv1.BeforeClusterDelete,
				Name:        strings.ToLower(h.Name()) + "-bcd",
				HandlerFunc: t.BeforeClusterDelete,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(mutation.DiscoverVariables); ok {
			if err := webhookServer.AddExtensionHandler(runtimeserver.ExtensionHandler{
				Hook:        runtimehooksv1.DiscoverVariables,
				Name:        strings.ToLower(h.Name()) + "-dv",
				HandlerFunc: t.DiscoverVariables,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(mutation.GeneratePatches); ok {
			if err := webhookServer.AddExtensionHandler(runtimeserver.ExtensionHandler{
				Hook:        runtimehooksv1.GeneratePatches,
				Name:        strings.ToLower(h.Name()) + "-gp",
				HandlerFunc: t.GeneratePatches,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(mutation.ValidateTopology); ok {
			if err := webhookServer.AddExtensionHandler(runtimeserver.ExtensionHandler{
				Hook:        runtimehooksv1.ValidateTopology,
				Name:        strings.ToLower(h.Name()) + "-vt",
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

// intToInt32Ptr converts an int to *int32 for use with TimeoutSeconds.
// #nosec G115 -- TimeoutSeconds is always within int32 safe range.
func intToInt32Ptr(value int) *int32 {
	t := int32(value)
	return &t
}
