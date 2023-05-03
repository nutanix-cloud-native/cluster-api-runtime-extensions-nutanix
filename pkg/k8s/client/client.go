// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type k8sResourcesApplyError struct {
	err error
}

func (e k8sResourcesApplyError) Error() string {
	return fmt.Sprintf("unable to apply Kubernetes resource: %v", e.err)
}

type GenericResourcesClient struct {
	client ctrlclient.Client
	log    logr.Logger
}

func NewGenericResourcesClient(client ctrlclient.Client, log logr.Logger) *GenericResourcesClient {
	return &GenericResourcesClient{
		client: client,
		log:    log,
	}
}

// Apply will apply objects via server-side apply. This will overwrite any changes that have been manually applied.
func (c *GenericResourcesClient) Apply(
	ctx context.Context,
	objs ...unstructured.Unstructured,
) error {
	for i := range objs {
		err := c.client.Patch(
			ctx,
			&objs[i],
			ctrlclient.Apply,
			ctrlclient.ForceOwnership,
			ctrlclient.FieldOwner("d2iq-capi-runtime-extensions"),
		)
		if err != nil {
			return k8sResourcesApplyError{err: err}
		}
	}

	return nil
}
