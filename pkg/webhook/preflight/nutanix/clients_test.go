// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"

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
