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

const (
	// AutoIPAMAdvertisementName is the L2Advertisement that announces the
	// per-Service pools the Nutanix CCM creates in automatic mode.
	AutoIPAMAdvertisementName = "nkp-auto-ipam"

	// autoIPAMPoolTypeLabel and autoIPAMPoolTypeValue are the contract with the
	// Nutanix CCM: it stamps this label on every IPAddressPool it creates for a
	// reserved Service address, and this advertisement selects on it.
	// Keep in sync with cloud-provider-nutanix internal/constants.
	autoIPAMPoolTypeLabel = "ccm.nutanix.com/load-balancer-pool-type"
	autoIPAMPoolTypeValue = "auto-ipam-metallb"
)

type ConfigurationInput struct {
	Name      string
	Namespace string

	// AddressRanges configures static mode. Empty means automatic mode.
	AddressRanges []v1alpha1.AddressRange

	// AutoIPAM configures automatic mode, where the CCM owns allocation and
	// creates one /32 IPAddressPool per Service.
	AutoIPAM *v1alpha1.ServiceLoadBalancerAutoIPAM
}

// ConfigurationObjects returns the MetalLB objects for the chosen address
// source. Exactly one source must be set; the ServiceLoadBalancer CRD enforces
// that, and this repeats it so a hand-built input cannot produce a cluster with
// two allocators or none.
func ConfigurationObjects(input *ConfigurationInput) ([]client.Object, error) {
	static, auto := len(input.AddressRanges) > 0, input.AutoIPAM != nil
	switch {
	case static && auto:
		return nil, fmt.Errorf("must define either AddressRanges or AutoIPAM, not both")
	case auto:
		return autoIPAMObjects(input)
	case static:
		return staticObjects(input)
	default:
		return nil, fmt.Errorf("must define one or more AddressRanges")
	}
}

// staticObjects announces an operator-maintained range. MetalLB owns allocation
// from the pool and the CCM Service controller stays out of the way.
func staticObjects(input *ConfigurationInput) ([]client.Object, error) {
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

// autoIPAMObjects announces the per-Service pools the CCM creates.
//
// No IPAddressPool is created here: in automatic mode the CCM reserves each
// address on Prism Central and creates a /32 pool for it. Creating a range pool
// as well would give the cluster a second allocator drawing from the same
// subnet with no knowledge of what Prism has reserved.
//
// The advertisement must match by label, not by name, because the pool names
// are Service UIDs that only exist at Service-create time. The selector is
// always non-empty: MetalLB treats an L2Advertisement with neither
// ipAddressPools nor ipAddressPoolSelectors as applying to every pool.
func autoIPAMObjects(input *ConfigurationInput) ([]client.Object, error) {
	if input.AutoIPAM.Subnet == "" {
		return nil, fmt.Errorf("AutoIPAM requires a subnet")
	}

	l2Advertisement := &metallbv1.L2Advertisement{
		TypeMeta: metav1.TypeMeta{
			Kind:       "L2Advertisement",
			APIVersion: metallbv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      AutoIPAMAdvertisementName,
			Namespace: input.Namespace,
		},
		Spec: metallbv1.L2AdvertisementSpec{
			IPAddressPoolSelectors: []metav1.LabelSelector{{
				MatchLabels: map[string]string{autoIPAMPoolTypeLabel: autoIPAMPoolTypeValue},
			}},
		},
	}

	return []client.Object{
		l2Advertisement,
	}, nil
}
