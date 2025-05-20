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
	prismv3 "github.com/nutanix-cloud-native/prism-go-client/v3"
	prismv4 "github.com/nutanix-cloud-native/prism-go-client/v4"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

func New(kclient ctrlclient.Client, cluster *clusterv1.Cluster) preflight.Checker {
	return &nutanixChecker{
		kclient: kclient,
		cluster: cluster,
	}
}

type nutanixChecker struct {
	kclient ctrlclient.Client
	cluster *clusterv1.Cluster

	nutanixClusterConfigSpec                           *carenv1.NutanixClusterConfigSpec
	nutanixWorkerNodeConfigSpecByMachineDeploymentName map[string]*carenv1.NutanixWorkerNodeConfigSpec

	credentials prismgoclient.Credentials
	v3client    *prismv3.Client
	v4client    *prismv4.Client

	log logr.Logger
}

func (n *nutanixChecker) Init(
	ctx context.Context,
) []preflight.Check {
	n.log = ctrl.LoggerFrom(ctx).WithName("preflight/nutanix")

	checks := []preflight.Check{
		// The configuration check must run first, because it initializes data used by all other checks,
		// and the credentials check second, because it initializes the Nutanix clients used by other checks.
		n.initNutanixConfiguration(),
		n.initCredentialsCheck(ctx),
	}

	checks = append(checks, n.initVMImageChecks()...)

	// Add more checks here as needed.

	return checks
}
