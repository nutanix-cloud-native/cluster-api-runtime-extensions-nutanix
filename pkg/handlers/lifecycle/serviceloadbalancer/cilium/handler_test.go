// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cilium

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ciliumv2 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	ciliumv2alpha1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/serviceloadbalancer"
)

func TestNewReturnsProviderImpl(t *testing.T) {
	t.Parallel()

	c := fake.NewClientBuilder().Build()
	var p serviceloadbalancer.ServiceLoadBalancerProvider = New(c, &Config{}, nil)
	assert.NotNil(t, p)
}

func newClusterWithConfig(t *testing.T, cni string, kpMode v1alpha1.KubeProxyMode) *clusterv1.Cluster {
	t.Helper()

	spec := &apivariables.ClusterConfigSpec{
		KubeProxy: &v1alpha1.KubeProxy{Mode: kpMode},
		Addons: &apivariables.Addons{
			CNI: &v1alpha1.CNI{Provider: cni},
		},
	}
	v, err := apivariables.MarshalToClusterVariable(v1alpha1.ClusterConfigVariableName, spec)
	require.NoError(t, err)

	return &clusterv1.Cluster{
		Spec: clusterv1.ClusterSpec{
			Topology: clusterv1.Topology{
				ClassRef:  clusterv1.ClusterClassRef{Name: "cc"},
				Version:   "v1.30.0",
				Variables: []clusterv1.ClusterVariable{*v},
			},
		},
	}
}

func fakeSchemeWithCilium(t *testing.T) *runtime.Scheme {
	t.Helper()

	s := runtime.NewScheme()
	require.NoError(t, ciliumv2.AddToScheme(s))
	require.NoError(t, ciliumv2alpha1.AddToScheme(s))
	require.NoError(t, apiextensionsv1.AddToScheme(s))
	require.NoError(t, clusterv1.AddToScheme(s))
	return s
}

func TestApply_NoConfigIsNoop(t *testing.T) {
	t.Parallel()

	c := fake.NewClientBuilder().WithScheme(fakeSchemeWithCilium(t)).Build()
	p := New(c, &Config{}, nil)
	err := p.Apply(
		context.Background(),
		v1alpha1.ServiceLoadBalancer{Provider: v1alpha1.ServiceLoadBalancerProviderCilium},
		newClusterWithConfig(t, v1alpha1.CNIProviderCilium, v1alpha1.KubeProxyModeDisabled),
		logr.Discard(),
	)
	assert.NoError(t, err)
}

func TestApply_RejectsNonCiliumCNI(t *testing.T) {
	t.Parallel()

	c := fake.NewClientBuilder().WithScheme(fakeSchemeWithCilium(t)).Build()
	p := New(c, &Config{}, nil)
	slb := v1alpha1.ServiceLoadBalancer{
		Provider: v1alpha1.ServiceLoadBalancerProviderCilium,
		Configuration: &v1alpha1.ServiceLoadBalancerConfiguration{
			AddressRanges: []v1alpha1.AddressRange{{Start: "10.0.0.1", End: "10.0.0.2"}},
		},
	}
	err := p.Apply(
		context.Background(),
		slb,
		newClusterWithConfig(t, v1alpha1.CNIProviderCalico, v1alpha1.KubeProxyModeDisabled),
		logr.Discard(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Cilium CNI")
}

func TestApply_RejectsKubeProxyEnabled(t *testing.T) {
	t.Parallel()

	c := fake.NewClientBuilder().WithScheme(fakeSchemeWithCilium(t)).Build()
	p := New(c, &Config{}, nil)
	slb := v1alpha1.ServiceLoadBalancer{
		Provider: v1alpha1.ServiceLoadBalancerProviderCilium,
		Configuration: &v1alpha1.ServiceLoadBalancerConfiguration{
			AddressRanges: []v1alpha1.AddressRange{{Start: "10.0.0.1", End: "10.0.0.2"}},
		},
	}
	err := p.Apply(
		context.Background(),
		slb,
		newClusterWithConfig(t, v1alpha1.CNIProviderCilium, v1alpha1.KubeProxyModeIPTables),
		logr.Discard(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kube-proxy")
}
