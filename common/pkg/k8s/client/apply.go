// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"

	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// ServerSideApply will apply (i.e. create or update) objects via server-side apply. This will overwrite any changes
// that have been manually applied.
func ServerSideApply(
	ctx context.Context,
	c ctrlclient.Client,
	objs ...ctrlclient.Object,
) error {
	for i := range objs {
		err := c.Patch(
			ctx,
			objs[i],
			ctrlclient.Apply,
			ctrlclient.ForceOwnership,
			ctrlclient.FieldOwner("d2iq-cluster-api-runtime-extensions-nutanix"),
		)
		if err != nil {
			return fmt.Errorf("server-side apply failed: %w", err)
		}
	}

	return nil
}
