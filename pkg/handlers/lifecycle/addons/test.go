// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"context"

	"github.com/go-logr/logr"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

type TestStrategy struct {
	err error
}

func NewTestStrategy(err error) *TestStrategy {
	return &TestStrategy{err: err}
}

func (s TestStrategy) Apply(
	ctx context.Context,
	cluster *clusterv1beta2.Cluster,
	defaultsNamespace string,
	log logr.Logger,
) error {
	return s.err
}
