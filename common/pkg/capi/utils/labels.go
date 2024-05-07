// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterctlv1 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
)

type LabelFn func(labels map[string]string)

func NewLabels(fs ...LabelFn) map[string]string {
	labels := map[string]string{}
	for _, f := range fs {
		f(labels)
	}
	return labels
}

func WithClusterName(clusterName string) LabelFn {
	return func(labels map[string]string) {
		labels[clusterv1.ClusterNameLabel] = clusterName
	}
}

func WithMove() LabelFn {
	return func(labels map[string]string) {
		labels[clusterctlv1.ClusterctlMoveLabel] = ""
	}
}
