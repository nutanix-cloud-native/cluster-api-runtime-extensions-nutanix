// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"context"

	"github.com/go-logr/logr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

type Applier interface {
	Apply(
		ctx context.Context,
		cluster *clusterv1.Cluster,
		defaultsNamespace string,
		log logr.Logger,
	) error
}
