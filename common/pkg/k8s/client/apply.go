// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"

	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// FieldOwner is the field manager name used for server-side apply.
	FieldOwner = "d2iq-cluster-api-runtime-extensions-nutanix"
)

// ForceOwnership is an convenience alias of the same option in the controller-runtime client.
var ForceOwnership = ctrlclient.ForceOwnership

// ServerSideApply will apply (i.e. create or update) objects via server-side apply.
func ServerSideApply(
	ctx context.Context,
	c ctrlclient.Client,
	obj ctrlclient.Object,
	opts ...ctrlclient.PatchOption,
) error {
	options := []ctrlclient.PatchOption{ctrlclient.FieldOwner(FieldOwner)}
	options = append(options, opts...)
	err := c.Patch(
		ctx,
		obj,
		ctrlclient.Apply,
		options...,
	)
	if err != nil {
		return fmt.Errorf("server-side apply failed: %w", err)
	}
	return nil
}
