//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WaitForResourcesInput is the input for WaitForResources.
type WaitForResourcesInput struct {
	Getter    framework.Getter
	Resources []client.Object
}

// WaitForResources waits until the resources are present.
func WaitForResources(
	ctx context.Context,
	input WaitForResourcesInput,
	intervals ...interface{},
) {
	start := time.Now()

	for _, obj := range input.Resources {
		obj = obj.DeepCopyObject().(client.Object)
		key := client.ObjectKeyFromObject(obj)
		capie2e.Byf("waiting for resource %s %s to be present",
			obj.GetObjectKind().GroupVersionKind(),
			key,
		)
		Logf("starting to wait for resource %s %s to become present",
			obj.GetObjectKind().GroupVersionKind(),
			key,
		)
		Eventually(func() (bool, error) {
			if err := input.Getter.Get(ctx, key, obj); err != nil {
				if apierrors.IsNotFound(err) {
					return false, nil
				}
				return false, err
			}
			return true, nil
		}, intervals...).Should(BeTrue(),
			fmt.Sprintf("Resource %s %s was not found",
				obj.GetObjectKind().GroupVersionKind(),
				key,
			),
		)
		Logf("Resource %s is now available, took %v",
			obj.GetObjectKind().GroupVersionKind(),
			key,
			time.Since(start),
		)
	}
}
