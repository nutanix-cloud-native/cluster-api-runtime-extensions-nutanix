// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"errors"
	"testing"
	"time"

	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	netv4 "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"
	prismv4 "github.com/nutanix/ntnx-api-golang-clients/prism-go-client/v4/models/prism/v4/config"
	vmmv4 "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/content"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
)

type mockKubeClient struct {
	ctrlclient.Client
	SubResourceClient ctrlclient.SubResourceClient
	getFunc           func(
		ctx context.Context,
		key ctrlclient.ObjectKey,
		obj ctrlclient.Object,
		opts ...ctrlclient.GetOption,
	) error
}

func (m *mockKubeClient) Get(
	ctx context.Context,
	key ctrlclient.ObjectKey,
	obj ctrlclient.Object,
	opts ...ctrlclient.GetOption,
) error {
	return m.getFunc(ctx, key, obj, opts...)
}

func (m *mockKubeClient) SubResource(subResource string) ctrlclient.SubResourceClient {
	return m.SubResourceClient
}

func TestCallWithContext(t *testing.T) {
	t.Parallel()
	testSuccessValue := "success"
	testError := errors.New("test error")

	tests := []struct {
		name        string
		ctx         func() (context.Context, context.CancelFunc)
		f           func() (string, error)
		wantVal     string
		wantErr     error
		cancelAfter time.Duration
	}{
		{
			name: "should return value on success",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.Background(), func() {}
			},
			f: func() (string, error) {
				return testSuccessValue, nil
			},
			wantVal: testSuccessValue,
			wantErr: nil,
		},
		{
			name: "should return error when function fails",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.Background(), func() {}
			},
			f: func() (string, error) {
				return "", testError
			},
			wantErr: testError,
		},
		{
			name: "should return context error when context is cancelled during execution",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			f: func() (string, error) {
				time.Sleep(100 * time.Millisecond)
				return testSuccessValue, nil
			},
			wantErr:     context.Canceled,
			cancelAfter: 10 * time.Millisecond,
		},
		{
			name: "should return context error when context is already cancelled",
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, func() {}
			},
			f: func() (string, error) {
				t.Log("this function should not have its result returned")
				return testSuccessValue, nil
			},
			wantErr: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := tt.ctx()
			defer cancel()

			if tt.cancelAfter > 0 {
				go func() {
					time.Sleep(tt.cancelAfter)
					cancel()
				}()
			}

			gotVal, gotErr := callWithContext(ctx, tt.f)

			if !errors.Is(gotErr, tt.wantErr) {
				t.Errorf("callWithContext() error = %v, wantErr %v", gotErr, tt.wantErr)
			}

			if gotVal != tt.wantVal {
				t.Errorf("callWithContext() gotVal = %s, wantVal %s", gotVal, tt.wantVal)
			}
		})
	}
}

func TestClientWrapper_ResolveProject(t *testing.T) {
	t.Parallel()

	id := &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new("project-uuid"),
	}
	c := &clientWrapper{
		ResolveProjectFunc: func(context.Context, *capxv1.NutanixResourceIdentifier, string) (*resolvedProject, error) {
			return &resolvedProject{UUID: "project-uuid", Name: "project"}, nil
		},
	}
	got, err := c.ResolveProject(context.Background(), id, "")
	require.NoError(t, err)
	assert.Equal(t, "project-uuid", got.UUID)
}

func TestClientWrapper_ResolveProjectMissingFunc(t *testing.T) {
	t.Parallel()

	c := &clientWrapper{}
	got, err := c.ResolveProject(context.Background(), nil, "")
	require.Error(t, err)
	assert.Nil(t, got)
}

func TestListClustersApiResponseDataSetValueAcceptsInternalV17Models(t *testing.T) {
	t.Parallel()

	resp := &clustermgmtv4.ListClustersApiResponse{}
	resp.Data = clustermgmtv4.NewOneOfListClustersApiResponseData()

	err := resp.Data.SetValue([]clustermgmtv4.Cluster{
		{
			ExtId: new("cluster-uuid"),
			Name:  new("cluster-name"),
		},
	})
	require.NoError(t, err)
}

func TestListSubnetsApiResponseDataSetValueAcceptsInternalV17Models(t *testing.T) {
	t.Parallel()

	resp := &netv4.ListSubnetsApiResponse{}
	resp.Data = netv4.NewOneOfListSubnetsApiResponseData()

	err := resp.Data.SetValue([]netv4.Subnet{
		{
			ExtId: new("subnet-uuid"),
			Name:  new("subnet-name"),
		},
	})
	require.NoError(t, err)
}

func TestGetImageApiResponseDataSetValueRequiresValueType(t *testing.T) {
	t.Parallel()

	image := &vmmv4.Image{
		ExtId:       new("image-uuid"),
		Name:        new("image-name"),
		ObjectType_: new("vmm.v4.content.Image"),
	}

	t.Run("passing the pointer fails", func(t *testing.T) {
		t.Parallel()

		resp := &vmmv4.GetImageApiResponse{}
		resp.Data = vmmv4.NewOneOfGetImageApiResponseData()

		err := resp.Data.SetValue(image)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not expected type")
	})

	t.Run("dereferencing the pointer succeeds", func(t *testing.T) {
		t.Parallel()

		resp := &vmmv4.GetImageApiResponse{}
		resp.Data = vmmv4.NewOneOfGetImageApiResponseData()

		err := resp.Data.SetValue(*image)
		require.NoError(t, err)

		gotImage, ok := resp.GetData().(vmmv4.Image)
		require.True(t, ok)
		assert.Equal(t, "image-uuid", *gotImage.ExtId)
	})
}

func TestClientWrapper_ListCategories(t *testing.T) {
	t.Parallel()

	c := &clientWrapper{
		ListCategoriesFunc: func(
			ctx context.Context,
			page_ *int,
			limit_ *int,
			filter_ *string,
			orderby_ *string,
			select_ *string,
			args ...map[string]any,
		) (*prismv4.ListCategoriesApiResponse, error) {
			resp := &prismv4.ListCategoriesApiResponse{}
			resp.Data = prismv4.NewOneOfListCategoriesApiResponseData()
			err := resp.Data.SetValue([]prismv4.Category{{Key: new("env"), Value: new("prod")}})
			require.NoError(t, err)
			return resp, nil
		},
	}

	resp, err := c.ListCategories(context.Background(), nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestClientWrapper_ListCategoriesMissingFunc(t *testing.T) {
	t.Parallel()

	c := &clientWrapper{}
	_, err := c.ListCategories(context.Background(), nil, nil, nil, nil, nil)
	require.Error(t, err)
}
