// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cilium

import (
	"fmt"

	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	ciliumv2 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	ciliumv2alpha1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

// ConfigurationInput is the pure input used to derive the workload-cluster
// Cilium LB objects.
type ConfigurationInput struct {
	Name          string
	Namespace     string
	AddressRanges []v1alpha1.AddressRange
}

// ConfigurationObjects returns the Cilium LoadBalancer IP pool and L2
// announcement policy in a deterministic order (pool first, policy second).
func ConfigurationObjects(input *ConfigurationInput) ([]ctrlclient.Object, error) {
	if len(input.AddressRanges) == 0 {
		return nil, fmt.Errorf("must define one or more AddressRanges")
	}

	pool := &ciliumv2.CiliumLoadBalancerIPPool{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CiliumLoadBalancerIPPool",
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      input.Name,
			Namespace: input.Namespace,
		},
		Spec: ciliumv2.CiliumLoadBalancerIPPoolSpec{
			Blocks: lo.Map(
				input.AddressRanges,
				func(ar v1alpha1.AddressRange, _ int) ciliumv2.CiliumLoadBalancerIPPoolIPBlock {
					return ciliumv2.CiliumLoadBalancerIPPoolIPBlock{
						Start: ar.Start,
						Stop:  ar.End,
					}
				},
			),
		},
	}

	policy := &ciliumv2alpha1.CiliumL2AnnouncementPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CiliumL2AnnouncementPolicy",
			APIVersion: ciliumv2alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      input.Name,
			Namespace: input.Namespace,
		},
		Spec: ciliumv2alpha1.CiliumL2AnnouncementPolicySpec{
			LoadBalancerIPs: true,
			ExternalIPs:     false,
		},
	}

	return []ctrlclient.Object{pool, policy}, nil
}
