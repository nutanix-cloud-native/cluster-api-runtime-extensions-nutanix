// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	runtimecatalog "sigs.k8s.io/cluster-api/exp/runtime/catalog"
	runtimeserver "sigs.k8s.io/cluster-api/exp/runtime/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
)

type ServerOptions struct {
	webhookPort    int
	webhookCertDir string
}

func NewServerOptions() *ServerOptions {
	return &ServerOptions{
		webhookPort: 9443,
	}
}

func (s *ServerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.IntVar(&s.webhookPort, "webhook-port", s.webhookPort, "Webhook Server port")

	fs.StringVar(
		&s.webhookCertDir,
		"webhook-cert-dir",
		s.webhookCertDir,
		"Webhook server cert dir.",
	)
}

// NewWebhookServer creates a CAPI runtime extension server that implements
// controller-runtime's webhook.Server. Use it as manager.Options.WebhookServer so
// admission and runtime hooks share one HTTPS listener. Call AddHandlers before
// manager.Start; do not Start the server separately.
func NewWebhookServer(opts *ServerOptions) (webhook.Server, error) {
	catalog := runtimecatalog.New()
	_ = runtimehooksv1.AddToCatalog(catalog)

	return runtimeserver.New(runtimeserver.Options{
		Catalog: catalog,
		Port:    opts.webhookPort,
		CertDir: opts.webhookCertDir,
	})
}

// AddHandlers registers runtime extension handlers on a server returned by
// NewWebhookServer. Path registration and listening happen in the server's Start,
// which the manager invokes.
func AddHandlers(s webhook.Server, hooks ...handlers.Named) error {
	rs, ok := s.(*runtimeserver.Server)
	if !ok {
		return fmt.Errorf("webhook server is %T, want *runtimeserver.Server", s)
	}

	for _, h := range hooks {
		if t, ok := h.(lifecycle.BeforeClusterCreate); ok {
			if err := rs.AddExtensionHandler(runtimeserver.ExtensionHandler{
				Hook:        runtimehooksv1.BeforeClusterCreate,
				Name:        strings.ToLower(h.Name()) + "-bcc",
				HandlerFunc: t.BeforeClusterCreate,
			}); err != nil {
				return err
			}
		}

		if t, ok := h.(lifecycle.AfterControlPlaneInitialized); ok {
			if err := rs.AddExtensionHandler(runtimeserver.ExtensionHandler{
				Hook:        runtimehooksv1.AfterControlPlaneInitialized,
				Name:        strings.ToLower(h.Name()) + "-acpi",
				HandlerFunc: t.AfterControlPlaneInitialized,
			}); err != nil {
				return err
			}
		}

		if t, ok := h.(lifecycle.BeforeClusterUpgrade); ok {
			if err := rs.AddExtensionHandler(runtimeserver.ExtensionHandler{
				Hook:        runtimehooksv1.BeforeClusterUpgrade,
				Name:        strings.ToLower(h.Name()) + "-bcu",
				HandlerFunc: t.BeforeClusterUpgrade,
			}); err != nil {
				return err
			}
		}

		if t, ok := h.(lifecycle.AfterControlPlaneUpgrade); ok {
			if err := rs.AddExtensionHandler(runtimeserver.ExtensionHandler{
				Hook:        runtimehooksv1.AfterControlPlaneUpgrade,
				Name:        h.Name() + "-acpu",
				HandlerFunc: t.AfterControlPlaneUpgrade,
			}); err != nil {
				return err
			}
		}

		if t, ok := h.(lifecycle.BeforeClusterDelete); ok {
			if err := rs.AddExtensionHandler(runtimeserver.ExtensionHandler{
				Hook:        runtimehooksv1.BeforeClusterDelete,
				Name:        strings.ToLower(h.Name()) + "-bcd",
				HandlerFunc: t.BeforeClusterDelete,
			}); err != nil {
				return err
			}
		}

		if t, ok := h.(mutation.DiscoverVariables); ok {
			if err := rs.AddExtensionHandler(runtimeserver.ExtensionHandler{
				Hook:        runtimehooksv1.DiscoverVariables,
				Name:        strings.ToLower(h.Name()) + "-dv",
				HandlerFunc: t.DiscoverVariables,
			}); err != nil {
				return err
			}
		}

		if t, ok := h.(mutation.GeneratePatches); ok {
			if err := rs.AddExtensionHandler(runtimeserver.ExtensionHandler{
				Hook:        runtimehooksv1.GeneratePatches,
				Name:        strings.ToLower(h.Name()) + "-gp",
				HandlerFunc: t.GeneratePatches,
			}); err != nil {
				return err
			}
		}

		if t, ok := h.(mutation.ValidateTopology); ok {
			if err := rs.AddExtensionHandler(runtimeserver.ExtensionHandler{
				Hook:        runtimehooksv1.ValidateTopology,
				Name:        strings.ToLower(h.Name()) + "-vt",
				HandlerFunc: t.ValidateTopology,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}
