// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"slices"
	"strings"

	"github.com/spf13/pflag"
	runtimecatalog "sigs.k8s.io/cluster-api/exp/runtime/catalog"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/server"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/handlers/lifecycle"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/handlers/mutation"
)

type Server struct {
	allExtensionHandlers []handlers.Named

	webhookPort    int
	webhookCertDir string

	catalog *runtimecatalog.Catalog

	enabledHandlers []string
}

func NewServer(extensionHandlers ...handlers.Named) *Server {
	// catalog contains all information about RuntimeHooks.
	catalog := runtimecatalog.New()

	_ = runtimehooksv1.AddToCatalog(catalog)

	return &Server{
		allExtensionHandlers: extensionHandlers,
		catalog:              catalog,
		webhookPort:          9443,
		webhookCertDir:       "/runtimehooks-certs/",
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

	handlerNames := make([]string, 0, len(s.allExtensionHandlers))
	for _, h := range s.allExtensionHandlers {
		handlerNames = append(handlerNames, h.Name())
	}

	fs.StringSliceVar(
		&s.enabledHandlers,
		prefix+".enabled-handlers",
		handlerNames,
		"list of all enabled handlers",
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

	for idx := range s.allExtensionHandlers {
		h := s.allExtensionHandlers[idx]

		if !slices.Contains(s.enabledHandlers, h.Name()) {
			continue
		}

		if t, ok := h.(lifecycle.BeforeClusterCreate); ok {
			if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
				Hook:        runtimehooksv1.BeforeClusterCreate,
				Name:        strings.ToLower(h.Name()),
				HandlerFunc: t.BeforeClusterCreate,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(lifecycle.AfterControlPlaneInitialized); ok {
			if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
				Hook:        runtimehooksv1.AfterControlPlaneInitialized,
				Name:        strings.ToLower(h.Name()),
				HandlerFunc: t.AfterControlPlaneInitialized,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(lifecycle.BeforeClusterUpgrade); ok {
			if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
				Hook:        runtimehooksv1.BeforeClusterUpgrade,
				Name:        strings.ToLower(h.Name()),
				HandlerFunc: t.BeforeClusterUpgrade,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(lifecycle.AfterControlPlaneUpgrade); ok {
			if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
				Hook:        runtimehooksv1.AfterControlPlaneUpgrade,
				Name:        h.Name(),
				HandlerFunc: t.AfterControlPlaneUpgrade,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(lifecycle.BeforeClusterDelete); ok {
			if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
				Hook:        runtimehooksv1.BeforeClusterDelete,
				Name:        strings.ToLower(h.Name()),
				HandlerFunc: t.BeforeClusterDelete,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(mutation.DiscoverVariables); ok {
			if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
				Hook:        runtimehooksv1.DiscoverVariables,
				Name:        strings.ToLower(h.Name()),
				HandlerFunc: t.DiscoverVariables,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(mutation.GeneratePatches); ok {
			if err := webhookServer.AddExtensionHandler(server.ExtensionHandler{
				Hook:        runtimehooksv1.GeneratePatches,
				Name:        strings.ToLower(h.Name()),
				HandlerFunc: t.GeneratePatches,
			}); err != nil {
				setupLog.Error(err, "error adding handler")
				return err
			}
		}

		if t, ok := h.(mutation.ValidateTopology); ok {
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
