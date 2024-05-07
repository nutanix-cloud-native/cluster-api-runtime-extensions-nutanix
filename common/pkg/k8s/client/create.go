// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"

	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func Create(
	ctx context.Context,
	c ctrlclient.Client,
	obj ctrlclient.Object,
	opts ...ctrlclient.CreateOption,
) error {
	options := []ctrlclient.CreateOption{ctrlclient.FieldOwner(FieldOwner)}
	options = append(options, opts...)
	err := c.Create(
		ctx,
		obj,
		options...,
	)
	if err != nil {
		return fmt.Errorf("create object failed: %w", err)
	}
	return nil
}
