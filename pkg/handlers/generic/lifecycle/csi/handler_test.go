// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package csi

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

type fakeCSIProvider struct {
	returnedErr error
}

func (p *fakeCSIProvider) Apply(
	ctx context.Context,
	provider v1alpha1.CSIProvider,
	defaultStorage v1alpha1.DefaultStorage,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	return p.returnedErr
}

var testProviderHandlers = map[string]CSIProvider{
	"test1": &fakeCSIProvider{},
	"test2": &fakeCSIProvider{},
	"broken": &fakeCSIProvider{
		returnedErr: fmt.Errorf("fake error"),
	},
}

func testReq(csi *apivariables.CSI) (*runtimehooksv1.AfterControlPlaneInitializedRequest, error) {
	cv, err := apivariables.MarshalToClusterVariable(
		"clusterConfig",
		&apivariables.ClusterConfigSpec{
			Addons: &apivariables.Addons{
				CSI: csi,
			},
		},
	)
	if err != nil {
		return nil, err
	}

	return &runtimehooksv1.AfterControlPlaneInitializedRequest{
		Cluster: clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				Topology: &clusterv1.Topology{
					Variables: []clusterv1.ClusterVariable{
						*cv,
					},
				},
			},
		},
	}, nil
}

func Test_AfterControlPlaneInitialized(t *testing.T) {
	tests := []struct {
		name       string
		csi        *apivariables.CSI
		wantStatus runtimehooksv1.ResponseStatus
	}{
		{
			name:       "csi variable is optional",
			csi:        nil,
			wantStatus: runtimehooksv1.ResponseStatusSuccess,
		},
		{
			name: "if csi variable set, must set at least one provider",
			csi: &apivariables.CSI{
				Providers: map[string]v1alpha1.CSIProvider{},
				GenericCSI: v1alpha1.GenericCSI{
					DefaultStorage: v1alpha1.DefaultStorage{
						Provider:           "test-provider",
						StorageClassConfig: "test-sc",
					},
				},
			},
			wantStatus: runtimehooksv1.ResponseStatusFailure,
		},
		{
			name: "default storage Provider must be the name of a configured storage class",
			csi: &apivariables.CSI{
				Providers: map[string]v1alpha1.CSIProvider{
					"test1": {
						StorageClassConfigs: map[string]v1alpha1.StorageClassConfig{
							"test1": {},
						},
					},
				},
				GenericCSI: v1alpha1.GenericCSI{
					DefaultStorage: v1alpha1.DefaultStorage{
						Provider:           "test1",
						StorageClassConfig: "test2",
					},
				},
			},
			wantStatus: runtimehooksv1.ResponseStatusFailure,
		},
		{
			name: "csi provider is unknown",
			csi: &apivariables.CSI{
				Providers: map[string]v1alpha1.CSIProvider{
					"not-test1-or-test2": {
						StorageClassConfigs: map[string]v1alpha1.StorageClassConfig{
							"not-test1-or-test2": {},
						},
					},
				},
				GenericCSI: v1alpha1.GenericCSI{
					DefaultStorage: v1alpha1.DefaultStorage{
						Provider:           "not-test1-or-test2",
						StorageClassConfig: "not-test1-or-test2",
					},
				},
			},
			wantStatus: runtimehooksv1.ResponseStatusFailure,
		},
		{
			name: "valid csi configuration",
			csi: &apivariables.CSI{
				Providers: map[string]v1alpha1.CSIProvider{
					"test1": {
						StorageClassConfigs: map[string]v1alpha1.StorageClassConfig{
							"test1": {},
						},
					},
					"test2": {
						StorageClassConfigs: map[string]v1alpha1.StorageClassConfig{
							"test2": {},
						},
					},
				},
				GenericCSI: v1alpha1.GenericCSI{
					DefaultStorage: v1alpha1.DefaultStorage{
						Provider:           "test2",
						StorageClassConfig: "test2",
					},
				},
			},
			wantStatus: runtimehooksv1.ResponseStatusSuccess,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := fake.NewClientBuilder().Build()
			handler := New(client, testProviderHandlers)
			resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}

			req, err := testReq(tt.csi)
			if err != nil {
				t.Fatalf("failed to create test request: %s", err)
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
