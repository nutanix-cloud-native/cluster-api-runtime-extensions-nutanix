// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"errors"
	"testing"
	"time"

	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
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
