// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanixflow

import (
	"bytes"
	"fmt"
	"text/template"

	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func templateValues(cluster *clusterv1beta2.Cluster, text string) (string, error) {
	t, err := template.New("").Parse(text)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	type input struct {
		ControlPlaneEndpoint clusterv1beta2.APIEndpoint
		PodCIDR              string
		ServiceCIDR          string
	}

	var podCIDR string
	if len(cluster.Spec.ClusterNetwork.Pods.CIDRBlocks) > 0 {
		podCIDR = cluster.Spec.ClusterNetwork.Pods.CIDRBlocks[0]
	}

	var serviceCIDR string
	if len(cluster.Spec.ClusterNetwork.Services.CIDRBlocks) > 0 {
		serviceCIDR = cluster.Spec.ClusterNetwork.Services.CIDRBlocks[0]
	}

	templateInput := input{
		ControlPlaneEndpoint: cluster.Spec.ControlPlaneEndpoint,
		PodCIDR:              podCIDR,
		ServiceCIDR:          serviceCIDR,
	}

	var b bytes.Buffer
	err = t.Execute(&b, templateInput)
	if err != nil {
		return "", fmt.Errorf("failed templating Nutanix Flow CNI values: %w", err)
	}

	return b.String(), nil
}
