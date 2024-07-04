// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metallb

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func GroupVersionKind(kind string) schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   "metallb.io",
		Version: "v1beta1",
		Kind:    kind,
	}
}

type ConfigurationInput struct {
	Name          string
	Namespace     string
	AddressRanges []v1alpha1.AddressRange
}

func ConfigurationObjects(input *ConfigurationInput) ([]*unstructured.Unstructured, error) {
	if len(input.AddressRanges) == 0 {
		return nil, fmt.Errorf("must define one or more AddressRanges")
	}

	ipAddressPool := &unstructured.Unstructured{}
	ipAddressPool.SetGroupVersionKind(GroupVersionKind("IPAddressPool"))
	ipAddressPool.SetName(input.Name)
	ipAddressPool.SetNamespace(input.Namespace)

	addresses := []string{}
	for _, ar := range input.AddressRanges {
		addresses = append(addresses, fmt.Sprintf("%s-%s", ar.Start, ar.End))
	}
	if err := unstructured.SetNestedStringSlice(
		ipAddressPool.Object,
		addresses,
		"spec",
		"addresses",
	); err != nil {
		return nil, fmt.Errorf("failed to set IPAddressPool .spec.addresses: %w", err)
	}

	l2Advertisement := &unstructured.Unstructured{}
	l2Advertisement.SetGroupVersionKind(GroupVersionKind("L2Advertisement"))
	l2Advertisement.SetName(input.Name)
	l2Advertisement.SetNamespace(input.Namespace)

	if err := unstructured.SetNestedStringSlice(
		l2Advertisement.Object,
		[]string{
			ipAddressPool.GetName(),
		},
		"spec",
		"ipAddressPools",
	); err != nil {
		return nil, fmt.Errorf("failed to set L2Advertisement .spec.ipAddressPools: %w", err)
	}

	return []*unstructured.Unstructured{
		ipAddressPool,
		l2Advertisement,
	}, nil
}
