// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package generic

import (
	"context"

	"github.com/go-logr/logr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

var Checker = &genericChecker{
	registryCheckFactory:      newRegistryCheck,
	configurationCheckFactory: newConfigurationCheck,
}

type genericChecker struct {
	configurationCheckFactory func(
		cd *checkDependencies,
	) preflight.Check
	registryCheckFactory func(
		cd *checkDependencies,
	) []preflight.Check
}

type checkDependencies struct {
	kclient                  ctrlclient.Client
	cluster                  *clusterv1.Cluster
	genericClusterConfigSpec *carenv1.GenericClusterConfigSpec
	log                      logr.Logger
}

func (g *genericChecker) Init(
	ctx context.Context,
	kclient ctrlclient.Client,
	cluster *clusterv1.Cluster,
) []preflight.Check {
	cd := &checkDependencies{
		kclient: kclient,
		cluster: cluster,
		log:     ctrl.LoggerFrom(ctx).WithName("preflight/generic"),
	}
	checks := []preflight.Check{
		// The configuration check must run first, because it initializes data used by all other checks.
		g.configurationCheckFactory(cd),
	}
	checks = append(checks, g.registryCheckFactory(cd)...)
	return checks
}
