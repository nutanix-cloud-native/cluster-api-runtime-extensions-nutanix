// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type k8sResourcesCreateError struct {
	err error
}

func (e k8sResourcesCreateError) Error() string {
	return fmt.Sprintf("unable to create kubernetes resource: %v", e.err)
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

// Create will create objects, ignoring individual already exists errors.
func (c *GenericResourcesClient) Create(
	ctx context.Context,
	objects []unstructured.Unstructured,
) error {
	opts := &ctrlclient.CreateOptions{}

	// try to create, continue if it is just an alreadyExists error, fail otherwise
	for i := range objects {
		err := c.client.Create(ctx, &objects[i], opts)
		if err != nil && !errors.IsAlreadyExists(err) {
			return k8sResourcesCreateError{err: err}
		}
	}

	return nil
}
