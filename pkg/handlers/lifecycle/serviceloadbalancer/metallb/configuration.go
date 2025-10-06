// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metallb

import (
	"fmt"

	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metallbv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/go.universe.tf/metallb/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

type ConfigurationInput struct {
	Name          string
	Namespace     string
	AddressRanges []v1alpha1.AddressRange
}

func ConfigurationObjects(input *ConfigurationInput) ([]client.Object, error) {
	if len(input.AddressRanges) == 0 {
		return nil, fmt.Errorf("must define one or more AddressRanges")
	}

	ipAddressPool := &metallbv1.IPAddressPool{
		TypeMeta: metav1.TypeMeta{
			Kind:       "IPAddressPool",
			APIVersion: metallbv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      input.Name,
			Namespace: input.Namespace,
		},
		Spec: metallbv1.IPAddressPoolSpec{
			Addresses: lo.Map(input.AddressRanges, func(ar v1alpha1.AddressRange, _ int) string {
				return fmt.Sprintf("%s-%s", ar.Start, ar.End)
			}),
		},
	}

	l2Advertisement := &metallbv1.L2Advertisement{
		TypeMeta: metav1.TypeMeta{
			Kind:       "L2Advertisement",
			APIVersion: metallbv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      input.Name,
			Namespace: input.Namespace,
		},
		Spec: metallbv1.L2AdvertisementSpec{
			IPAddressPools: []string{ipAddressPool.GetName()},
		},
	}

	return []client.Object{
		ipAddressPool,
		l2Advertisement,
	}, nil
}
