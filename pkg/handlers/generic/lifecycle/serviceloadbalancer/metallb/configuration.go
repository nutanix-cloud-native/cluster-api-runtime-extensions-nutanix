package metallb

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func groupVersionKind(kind string) schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   "metallb.io",
		Version: "v1beta1",
		Kind:    kind,
	}
}

type configurationInput struct {
	name          string
	namespace     string
	addressRanges []v1alpha1.AddressRange
}

func configurationObjects(input *configurationInput) ([]unstructured.Unstructured, error) {
	ipAddressPool := unstructured.Unstructured{}
	ipAddressPool.SetGroupVersionKind(groupVersionKind("IPAddressPool"))
	ipAddressPool.SetName(input.name)
	ipAddressPool.SetNamespace(input.namespace)

	addresses := []string{}
	for _, ar := range input.addressRanges {
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

	l2Advertisement := unstructured.Unstructured{}
	l2Advertisement.SetGroupVersionKind(groupVersionKind("L2Advertisement"))
	l2Advertisement.SetName(input.name)
	l2Advertisement.SetNamespace(input.namespace)

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

	return []unstructured.Unstructured{
		ipAddressPool,
		l2Advertisement,
	}, nil
}
