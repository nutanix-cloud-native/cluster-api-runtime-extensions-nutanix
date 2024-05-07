// Copyright 2024 D2iQ, Inc. All rights reserved.
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
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

type fakeCSIProvider struct {
	returnedErr error
}

func (p *fakeCSIProvider) Apply(
	ctx context.Context,
	provider v1alpha1.CSIProvider,
	defaultStorageConfig v1alpha1.DefaultStorage,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
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

func testReq(csi *v1alpha1.CSI) (*runtimehooksv1.AfterControlPlaneInitializedRequest, error) {
	cv, err := variables.MarshalToClusterVariable(
		"clusterConfig",
		&v1alpha1.GenericClusterConfigSpec{
			Addons: &v1alpha1.Addons{
				CSIProviders: csi,
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
		csi        *v1alpha1.CSI
		wantStatus runtimehooksv1.ResponseStatus
	}{
		{
			name:       "csi variable is optional",
			csi:        nil,
			wantStatus: runtimehooksv1.ResponseStatusSuccess,
		},
		{
			name: "if csi variable set, must set at least one provider",
			csi: &v1alpha1.CSI{
				Providers: []v1alpha1.CSIProvider{},
				DefaultStorage: v1alpha1.DefaultStorage{
					ProviderName:           "example",
					StorageClassConfigName: "example",
				},
			},
			wantStatus: runtimehooksv1.ResponseStatusFailure,
		},
		{
			name: "default storage Provider name must be the name of a set provider",
			csi: &v1alpha1.CSI{
				Providers: []v1alpha1.CSIProvider{
					{
						Name: "test1",
						StorageClassConfig: []v1alpha1.StorageClassConfig{
							{
								Name: "test1",
							},
						},
					},
				},
				DefaultStorage: v1alpha1.DefaultStorage{
					ProviderName:           "not-test1",
					StorageClassConfigName: "test1",
				},
			},
			wantStatus: runtimehooksv1.ResponseStatusFailure,
		},
		{
			name: "default storage StorageClassConfig name must be the name of a StorageClassConfig of the default provider",
			csi: &v1alpha1.CSI{
				Providers: []v1alpha1.CSIProvider{
					{
						Name: "test1",
						StorageClassConfig: []v1alpha1.StorageClassConfig{
							{
								Name: "test1",
							},
						},
					},
					{
						Name: "test2",
						StorageClassConfig: []v1alpha1.StorageClassConfig{
							{
								Name: "test2",
							},
						},
					},
				},
				DefaultStorage: v1alpha1.DefaultStorage{
					ProviderName:           "test1",
					StorageClassConfigName: "test2",
				},
			},
			wantStatus: runtimehooksv1.ResponseStatusFailure,
		},
		{
			name: "csi provider is unknown",
			csi: &v1alpha1.CSI{
				Providers: []v1alpha1.CSIProvider{
					{
						Name: "not-test1-or-test2",
						StorageClassConfig: []v1alpha1.StorageClassConfig{
							{
								Name: "not-test1-or-test2",
							},
						},
					},
				},
				DefaultStorage: v1alpha1.DefaultStorage{
					ProviderName:           "not-test1-or-test2",
					StorageClassConfigName: "not-test1-or-test2",
				},
			},
			wantStatus: runtimehooksv1.ResponseStatusFailure,
		},
		{
			name: "valid csi configuration",
			csi: &v1alpha1.CSI{
				Providers: []v1alpha1.CSIProvider{
					{
						Name: "test1",
						StorageClassConfig: []v1alpha1.StorageClassConfig{
							{
								Name: "test1",
							},
						},
					},
					{
						Name: "test2",
						StorageClassConfig: []v1alpha1.StorageClassConfig{
							{
								Name: "test2",
							},
						},
					},
				},
				DefaultStorage: v1alpha1.DefaultStorage{
					ProviderName:           "test2",
					StorageClassConfigName: "test2",
				},
			},
			wantStatus: runtimehooksv1.ResponseStatusSuccess,
		},
		{
			name: "csi handler fails to apply",
			csi: &v1alpha1.CSI{
				Providers: []v1alpha1.CSIProvider{
					{
						Name: "broken",
						StorageClassConfig: []v1alpha1.StorageClassConfig{
							{
								Name: "broken",
							},
						},
					},
				},
				DefaultStorage: v1alpha1.DefaultStorage{
					ProviderName:           "broken",
					StorageClassConfigName: "broken",
				},
			},
			wantStatus: runtimehooksv1.ResponseStatusFailure,
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
