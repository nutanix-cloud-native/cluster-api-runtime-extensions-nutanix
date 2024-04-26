// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package providers

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

const (
	kubeVIPFileOwner       = "root:root"
	kubeVIPFilePath        = "/etc/kubernetes/manifests/kube-vip.yaml"
	kubeVIPFilePermissions = "0600"
)

// Provider is an interface for getting the kube-vip static Pod as a file.
type Provider interface {
	Name() string
	GetFile(ctx context.Context, spec v1alpha1.ControlPlaneEndpointSpec) (*bootstrapv1.File, error)
	GetCommands(cluster *clusterv1.Cluster) ([]string, []string, error)
}

func templateValues(
	controlPlaneEndpoint v1alpha1.ControlPlaneEndpointSpec,
	text string,
) (string, error) {
	kubeVIPTemplate, err := template.New("").Parse(text)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	type input struct {
		ControlPlaneEndpoint v1alpha1.ControlPlaneEndpointSpec
	}

	templateInput := input{
		ControlPlaneEndpoint: controlPlaneEndpoint,
	}

	var b bytes.Buffer
	err = kubeVIPTemplate.Execute(&b, templateInput)
	if err != nil {
		return "", fmt.Errorf("failed setting API endpoint configuration in template: %w", err)
	}

	return b.String(), nil
}
