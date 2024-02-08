// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package fakessa

import (
	"context"
	"errors"
	"fmt"

	"github.com/avast/retry-go/v4"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func NewClient(_ *rest.Config, _ client.Options) (client.Client, error) {
	return fake.NewClientBuilder().WithInterceptorFuncs(
		interceptor.Funcs{
			Patch: func(
				ctx context.Context,
				clnt client.WithWatch,
				obj client.Object,
				patch client.Patch,
				opts ...client.PatchOption,
			) error {
				// Apply patches are supposed to upsert, but fake client fails if the object doesn't exist,
				// if an apply patch occurs for an object that doesn't yet exist, create it.
				if patch.Type() != types.ApplyPatchType {
					return clnt.Patch(ctx, obj, patch, opts...)
				}
				check, ok := obj.DeepCopyObject().(client.Object)
				if !ok {
					return errors.New("could not check for object in fake client")
				}

				return retry.Do(
					func() error {
						if err := clnt.Get(ctx, client.ObjectKeyFromObject(obj), check); k8serrors.IsNotFound(
							err,
						) {
							if err := clnt.Create(ctx, check); err != nil {
								return fmt.Errorf(
									"could not inject object creation for fake: %w",
									err,
								)
							}
						}
						return clnt.Patch(ctx, obj, patch, opts...)
					},
					retry.Attempts(5),
				)
			},
		},
	).Build(), nil
}
