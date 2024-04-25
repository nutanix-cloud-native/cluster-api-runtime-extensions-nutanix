// Copyright 2023 D2iQ, Inc. All rights reserved.
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
	err := c.Patch(
		ctx,
		obj,
		ctrlclient.Apply,
		ctrlclient.FieldOwner(FieldOwner),
	)
	if err != nil {
		return fmt.Errorf("server-side apply failed: %w", err)
	}
	return nil
}
