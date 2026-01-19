// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"

	"github.com/go-logr/logr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

var Checker = &nutanixChecker{
	configurationCheckFactory:             newConfigurationCheck,
	credentialsCheckFactory:               newCredentialsCheck,
	prismCentralVersionCheckFactory:       newPrismCentralVersionCheck,
	failureDomainCheckFactory:             newFailureDomainChecks,
	vmImageChecksFactory:                  newVMImageChecks,
	vmImageKubernetesVersionChecksFactory: newVMImageKubernetesVersionChecks,
	storageContainerChecksFactory:         newStorageContainerChecks,
}

type nutanixChecker struct {
	configurationCheckFactory func(
		cd *checkDependencies,
	) preflight.Check

	credentialsCheckFactory func(
		ctx context.Context,
		nclientFactory func(prismgoclient.Credentials) (client, error),
		cd *checkDependencies,
	) preflight.Check

	prismCentralVersionCheckFactory func(
		ctx context.Context,
		cd *checkDependencies,
	) preflight.Check

	failureDomainCheckFactory func(
		cd *checkDependencies,
	) []preflight.Check

	vmImageChecksFactory func(
		cd *checkDependencies,
	) []preflight.Check

	vmImageKubernetesVersionChecksFactory func(
		cd *checkDependencies,
	) []preflight.Check

	storageContainerChecksFactory func(
		cd *checkDependencies,
	) []preflight.Check
}

type checkDependencies struct {
	kclient ctrlclient.Client
	cluster *clusterv1.Cluster

	nutanixClusterConfigSpec                           *carenv1.NutanixClusterConfigSpec
	nutanixWorkerNodeConfigSpecByMachineDeploymentName map[string]*carenv1.NutanixWorkerNodeConfigSpec
	failureDomainByMachineDeploymentName               map[string]string

	nclient   client
	pcVersion string
	log       logr.Logger
}

func (n *nutanixChecker) Init(
	ctx context.Context,
	kclient ctrlclient.Client,
	cluster *clusterv1.Cluster,
) []preflight.Check {
	cd := &checkDependencies{
		kclient:   kclient,
		cluster:   cluster,
		log:       ctrl.LoggerFrom(ctx).WithName("preflight/nutanix"),
		pcVersion: "",
	}

	checks := []preflight.Check{
		// The configuration check must run first, because it initializes data used by all other checks,
		// and the credentials check second, because it initializes the Nutanix clients used by other checks.
		n.configurationCheckFactory(cd),
		n.credentialsCheckFactory(ctx, newClient, cd),
		n.prismCentralVersionCheckFactory(ctx, cd),
	}

	checks = append(checks, n.failureDomainCheckFactory(cd)...)
	checks = append(checks, n.vmImageChecksFactory(cd)...)
	checks = append(checks, n.vmImageKubernetesVersionChecksFactory(cd)...)
	checks = append(checks, n.storageContainerChecksFactory(cd)...)

	// Add more checks here as needed.

	return checks
}
