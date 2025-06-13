// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"

	"github.com/go-logr/logr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

func New(kclient ctrlclient.Client, cluster *clusterv1.Cluster) preflight.Checker {
	return &nutanixChecker{
		kclient: kclient,
		cluster: cluster,

		nclientFactory: newClient,

		initNutanixConfigurationFunc: initNutanixConfiguration,
		initCredentialsCheckFunc:     initCredentialsCheck,
		initVMImageChecksFunc:        initVMImageChecks,
	}
}

type nutanixChecker struct {
	kclient ctrlclient.Client
	cluster *clusterv1.Cluster

	nutanixClusterConfigSpec                           *carenv1.NutanixClusterConfigSpec
	nutanixWorkerNodeConfigSpecByMachineDeploymentName map[string]*carenv1.NutanixWorkerNodeConfigSpec

	nclient        client
	nclientFactory func(prismgoclient.Credentials) (client, error)

	initNutanixConfigurationFunc func(
		n *nutanixChecker,
	) preflight.Check

	initCredentialsCheckFunc func(
		ctx context.Context,
		n *nutanixChecker,
	) preflight.Check

	initVMImageChecksFunc func(
		n *nutanixChecker,
	) []preflight.Check

	log logr.Logger
}

func (n *nutanixChecker) Init(
	ctx context.Context,
) []preflight.Check {
	n.log = ctrl.LoggerFrom(ctx).WithName("preflight/nutanix")

	checks := []preflight.Check{
		// The configuration check must run first, because it initializes data used by all other checks,
		// and the credentials check second, because it initializes the Nutanix clients used by other checks.
		n.initNutanixConfigurationFunc(n),
		n.initCredentialsCheckFunc(ctx, n),
	}

	checks = append(checks, n.initVMImageChecksFunc(n)...)

	// Add more checks here as needed.

	return checks
}
