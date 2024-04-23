// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package virtualip

import (
	"context"
	"fmt"
	"io"
	"strings"

	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

type fromReaderProvider struct {
	template string
}

func NewFromReaderProvider(reader io.Reader) (*fromReaderProvider, error) {
	buf := new(strings.Builder)
	_, err := io.Copy(buf, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to copy content from reader: %w", err)
	}

	return &fromReaderProvider{
		template: buf.String(),
	}, nil
}

func (p *fromReaderProvider) GetFile(
	_ context.Context,
	spec v1alpha1.ControlPlaneEndpointSpec,
) (*bootstrapv1.File, error) {
	virtualIPStaticPod, err := templateValues(spec, p.template)
	if err != nil {
		return nil, fmt.Errorf("failed templating static Pod: %w", err)
	}

	return &bootstrapv1.File{
		Content:     virtualIPStaticPod,
		Owner:       kubeVIPFileOwner,
		Path:        kubeVIPFilePath,
		Permissions: kubeVIPFilePermissions,
	}, nil
}
