// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package providers

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"text/template"

	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

var VirtualIPProviderFileNames = []string{
	kubeVIPFilePath,
}

// Provider is an interface for getting the virtual IP provider static Pod as a file.
type Provider interface {
	Name() string
	GenerateFilesAndCommands(
		ctx context.Context,
		spec v1alpha1.ControlPlaneEndpointSpec,
		cluster *clusterv1.Cluster,
	) ([]bootstrapv1.File, []string, []string, error)
}

func templateValues(
	controlPlaneEndpoint v1alpha1.ControlPlaneEndpointSpec,
	text string,
) (string, error) {
	virtualIPTemplate, err := template.New("").Parse(text)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	type input struct {
		Address string
		Port    int32
	}

	// If specified, use the virtual IP address and/or port,
	// otherwise fall back to the control plane endpoint host and port.
	var virtualIPConfig v1alpha1.ControlPlaneVirtualIPConfiguration
	if controlPlaneEndpoint.VirtualIPSpec != nil &&
		controlPlaneEndpoint.VirtualIPSpec.Configuration != nil {
		virtualIPConfig = *controlPlaneEndpoint.VirtualIPSpec.Configuration
	}
	templateInput := input{
		Address: cmp.Or(virtualIPConfig.Address, controlPlaneEndpoint.Host),
		Port:    cmp.Or(virtualIPConfig.Port, controlPlaneEndpoint.Port),
	}

	var b bytes.Buffer
	err = virtualIPTemplate.Execute(&b, templateInput)
	if err != nil {
		return "", fmt.Errorf("failed setting API endpoint configuration in template: %w", err)
	}

	return b.String(), nil
}
