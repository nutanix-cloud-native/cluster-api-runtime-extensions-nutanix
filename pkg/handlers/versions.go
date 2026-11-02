// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	awsclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/clusterconfig"
	awsworkerconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/workerconfig"
	dockerclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/clusterconfig"
	dockerworkerconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/workerconfig"
	eksclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/eks/clusterconfig"
	eksworkerconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/eks/workerconfig"
	genericclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
	nutanixworkerconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/workerconfig"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v7"
)

// AllHandlers returns the variable discovery handlers and the topology mutation handlers of every served version.
func AllHandlers(mgr manager.Manager) []handlers.Named {
	return slices.Concat(
		[]handlers.Named{
			awsclusterconfig.NewVariable(),
			awsworkerconfig.NewVariable(),
			dockerclusterconfig.NewVariable(),
			dockerworkerconfig.NewVariable(),
			eksclusterconfig.NewVariable(),
			eksworkerconfig.NewVariable(),
			genericclusterconfig.NewVariable(),
			nutanixclusterconfig.NewVariable(),
			nutanixworkerconfig.NewVariable(),
		},
		v6.AllHandlers(mgr),
		v7.AllHandlers(mgr),
		// new-version:entries
	)
}
