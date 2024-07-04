// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package serviceloadbalancer

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

type fakeServiceLoadBalancerProvider struct {
	returnedErr error
}

func (p *fakeServiceLoadBalancerProvider) Apply(
	ctx context.Context,
	slb v1alpha1.ServiceLoadBalancer,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	return p.returnedErr
}

var testProviderHandlers = map[string]ServiceLoadBalancerProvider{
	"test1": &fakeServiceLoadBalancerProvider{},
	"test2": &fakeServiceLoadBalancerProvider{},
	"broken": &fakeServiceLoadBalancerProvider{
		returnedErr: fmt.Errorf("fake error"),
	},
}

func testClusterVariable(
	t *testing.T,
	slb *v1alpha1.ServiceLoadBalancer,
) *clusterv1.ClusterVariable {
	t.Helper()
	cv, err := apivariables.MarshalToClusterVariable(
		"clusterConfig",
		&apivariables.ClusterConfigSpec{
			Addons: &apivariables.Addons{
				GenericAddons: v1alpha1.GenericAddons{
					ServiceLoadBalancer: slb,
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("failed to create clusterVariable: %s", err)
	}
	return cv
}

type testCase struct {
	name            string
	clusterVariable *clusterv1.ClusterVariable
	wantStatus      runtimehooksv1.ResponseStatus
}

func testCases(t *testing.T) []testCase {
	t.Helper()
	return []testCase{
		{
			name: "request is missing serviceLoadBalancer field",
			clusterVariable: testClusterVariable(
				t,
				nil,
			),
			wantStatus: runtimehooksv1.ResponseStatus(""), // Neither success, nor failure.
		},
		{
			name: "request is malformed",
			clusterVariable: &clusterv1.ClusterVariable{
				Name: "clusterConfig",
				Value: apiextensionsv1.JSON{
					Raw: []byte("{\"addons\":{\"serviceLoadBalancer\":{\"provider\": %%% }}}"),
				},
			},
			wantStatus: runtimehooksv1.ResponseStatusFailure,
		},
		{
			name: "provider is not known",
			clusterVariable: testClusterVariable(
				t,
				&v1alpha1.ServiceLoadBalancer{
					Provider: "unknown",
				},
			),
			wantStatus: runtimehooksv1.ResponseStatusFailure,
		},
		{
			name: "provider is known, deploy succeeds",
			clusterVariable: testClusterVariable(
				t,
				&v1alpha1.ServiceLoadBalancer{
					Provider: "test1",
				},
			),
			wantStatus: runtimehooksv1.ResponseStatusSuccess,
		},
		{
			name: "provider is known, deploy fails",
			clusterVariable: testClusterVariable(
				t,
				&v1alpha1.ServiceLoadBalancer{
					Provider: "broken",
				},
			),
			wantStatus: runtimehooksv1.ResponseStatusFailure,
		},
	}
}

func TestAfterControlPlaneInitialized(t *testing.T) {
	for _, tt := range testCases(t) {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := fake.NewClientBuilder().Build()
			handler := New(client, testProviderHandlers)
			resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}

			req := &runtimehooksv1.AfterControlPlaneInitializedRequest{
				Cluster: clusterv1.Cluster{
					Spec: clusterv1.ClusterSpec{
						Topology: &clusterv1.Topology{
							Variables: []clusterv1.ClusterVariable{
								*tt.clusterVariable,
							},
						},
					},
				},
			}

			handler.AfterControlPlaneInitialized(ctx, req, resp)
			if diff := cmp.Diff(tt.wantStatus, resp.Status); diff != "" {
				t.Errorf(
					"response Status mismatch (-want +got):\n%s. Message: %s",
					diff,
					resp.Message,
				)
			}
		})
	}
}

func TestBeforeClusterUpgrade(t *testing.T) {
	for _, tt := range testCases(t) {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := fake.NewClientBuilder().Build()
			handler := New(client, testProviderHandlers)
			resp := &runtimehooksv1.BeforeClusterUpgradeResponse{}

			req := &runtimehooksv1.BeforeClusterUpgradeRequest{
				Cluster: clusterv1.Cluster{
					Spec: clusterv1.ClusterSpec{
						Topology: &clusterv1.Topology{
							Variables: []clusterv1.ClusterVariable{
								*tt.clusterVariable,
							},
						},
					},
				},
			}

			handler.BeforeClusterUpgrade(ctx, req, resp)
			if diff := cmp.Diff(tt.wantStatus, resp.Status); diff != "" {
				t.Errorf(
					"response Status mismatch (-want +got):\n%s. Message: %s",
					diff,
					resp.Message,
				)
			}
		})
	}
}
