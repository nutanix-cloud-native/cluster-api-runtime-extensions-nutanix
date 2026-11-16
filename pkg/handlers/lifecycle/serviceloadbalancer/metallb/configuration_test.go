// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metallb

import (
	"testing"

	metallbv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/go.universe.tf/metallb/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func staticInput() *ConfigurationInput {
	return &ConfigurationInput{
		Name:      DefaultHelmReleaseName,
		Namespace: DefaultHelmReleaseNamespace,
		AddressRanges: []v1alpha1.AddressRange{
			{Start: "10.0.0.10", End: "10.0.0.20"},
		},
	}
}

func autoInput() *ConfigurationInput {
	return &ConfigurationInput{
		Name:      DefaultHelmReleaseName,
		Namespace: DefaultHelmReleaseNamespace,
		AutoIPAM:  &v1alpha1.ServiceLoadBalancerAutoIPAM{Subnet: "subnet-uuid"},
	}
}

func TestStaticModeAdvertisesTheRangePoolByName(t *testing.T) {
	objs, err := ConfigurationObjects(staticInput())
	if err != nil {
		t.Fatalf("ConfigurationObjects: %v", err)
	}
	if len(objs) != 2 {
		t.Fatalf("objects = %d, want an IPAddressPool and an L2Advertisement", len(objs))
	}

	pool, ok := objs[0].(*metallbv1.IPAddressPool)
	if !ok {
		t.Fatalf("objs[0] = %T, want *IPAddressPool", objs[0])
	}
	if len(pool.Spec.Addresses) != 1 || pool.Spec.Addresses[0] != "10.0.0.10-10.0.0.20" {
		t.Fatalf("pool addresses = %v", pool.Spec.Addresses)
	}

	adv, ok := objs[1].(*metallbv1.L2Advertisement)
	if !ok {
		t.Fatalf("objs[1] = %T, want *L2Advertisement", objs[1])
	}
	if len(adv.Spec.IPAddressPools) != 1 || adv.Spec.IPAddressPools[0] != pool.GetName() {
		t.Fatalf("advertisement pools = %v, want [%s]", adv.Spec.IPAddressPools, pool.GetName())
	}
	if len(adv.Spec.IPAddressPoolSelectors) != 0 {
		t.Fatalf("static mode set a selector: %v", adv.Spec.IPAddressPoolSelectors)
	}
}

// In automatic mode the CCM creates one /32 pool per Service. A range pool here
// would be a second allocator on the same subnet with no knowledge of what
// Prism Central has reserved.
func TestAutoIPAMModeCreatesNoAddressPool(t *testing.T) {
	objs, err := ConfigurationObjects(autoInput())
	if err != nil {
		t.Fatalf("ConfigurationObjects: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("objects = %d, want only an L2Advertisement", len(objs))
	}
	if pool, ok := objs[0].(*metallbv1.IPAddressPool); ok {
		t.Fatalf("automatic mode created an IPAddressPool: %v", pool.Spec.Addresses)
	}
}

// The pool names are Service UIDs that do not exist yet, so the advertisement
// has to select by label. The selector must never be empty: MetalLB applies an
// L2Advertisement with no pools and no selectors to every pool in the cluster.
func TestAutoIPAMModeAdvertisesByPoolTypeLabel(t *testing.T) {
	objs, err := ConfigurationObjects(autoInput())
	if err != nil {
		t.Fatalf("ConfigurationObjects: %v", err)
	}

	adv, ok := objs[0].(*metallbv1.L2Advertisement)
	if !ok {
		t.Fatalf("objs[0] = %T, want *L2Advertisement", objs[0])
	}
	if adv.GetName() != AutoIPAMAdvertisementName {
		t.Fatalf("advertisement name = %q, want %q", adv.GetName(), AutoIPAMAdvertisementName)
	}
	if len(adv.Spec.IPAddressPools) != 0 {
		t.Fatalf("automatic mode pinned pool names: %v", adv.Spec.IPAddressPools)
	}
	if len(adv.Spec.IPAddressPoolSelectors) != 1 {
		t.Fatalf("selectors = %v, want exactly one", adv.Spec.IPAddressPoolSelectors)
	}
	got := adv.Spec.IPAddressPoolSelectors[0].MatchLabels
	if got[autoIPAMPoolTypeLabel] != autoIPAMPoolTypeValue {
		t.Fatalf("selector = %v, want %s=%s", got, autoIPAMPoolTypeLabel, autoIPAMPoolTypeValue)
	}
	if len(adv.Spec.IPAddressPoolSelectors[0].MatchExpressions) != 0 {
		t.Fatalf("unexpected match expressions: %v", adv.Spec.IPAddressPoolSelectors[0].MatchExpressions)
	}
}

// The label is a cross-repo contract with cloud-provider-nutanix. Pin the
// literals so a rename on either side fails here instead of silently leaving
// reserved addresses unadvertised.
func TestAutoIPAMPoolTypeLabelContract(t *testing.T) {
	if autoIPAMPoolTypeLabel != "ccm.nutanix.com/load-balancer-pool-type" {
		t.Fatalf("pool-type label = %q", autoIPAMPoolTypeLabel)
	}
	if autoIPAMPoolTypeValue != "auto-ipam-metallb" {
		t.Fatalf("pool-type value = %q", autoIPAMPoolTypeValue)
	}
}

func TestConfigurationObjectsRejectsAmbiguousInput(t *testing.T) {
	both := staticInput()
	both.AutoIPAM = &v1alpha1.ServiceLoadBalancerAutoIPAM{Subnet: "subnet-uuid"}

	for name, input := range map[string]*ConfigurationInput{
		"neither": {Name: DefaultHelmReleaseName, Namespace: DefaultHelmReleaseNamespace},
		"both":    both,
		"auto without subnet": {
			Name:      DefaultHelmReleaseName,
			Namespace: DefaultHelmReleaseNamespace,
			AutoIPAM:  &v1alpha1.ServiceLoadBalancerAutoIPAM{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			objs, err := ConfigurationObjects(input)
			if err == nil {
				t.Fatalf("expected an error, got %d objects", len(objs))
			}
			if objs != nil {
				t.Fatalf("objects returned alongside an error: %v", objs)
			}
		})
	}
}

func TestConfigurationObjectsUseTheReleaseNamespace(t *testing.T) {
	for name, input := range map[string]*ConfigurationInput{
		"static": staticInput(),
		"auto":   autoInput(),
	} {
		t.Run(name, func(t *testing.T) {
			objs, err := ConfigurationObjects(input)
			if err != nil {
				t.Fatalf("ConfigurationObjects: %v", err)
			}
			for _, o := range objs {
				if o.GetNamespace() != DefaultHelmReleaseNamespace {
					t.Fatalf("%T namespace = %q, want %q", o, o.GetNamespace(), DefaultHelmReleaseNamespace)
				}
			}
		})
	}
}
