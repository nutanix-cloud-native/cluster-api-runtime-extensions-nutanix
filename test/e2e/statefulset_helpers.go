//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"time"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type WaitForStatefulSetAvailableInput struct {
	Getter      framework.Getter
	StatefulSet *appsv1.StatefulSet
}

// WaitForStatefulSetsAvailable waits until the Deployment has observedGeneration equal to generation and
// status.Available = True, that signals that all the desired replicas are in place.
func WaitForStatefulSetsAvailable(
	ctx context.Context, input WaitForStatefulSetAvailableInput, intervals ...interface{},
) {
	start := time.Now()
	key := client.ObjectKeyFromObject(input.StatefulSet)
	capie2e.Byf("waiting for statefulset %s to be available", key)
	Log("starting to wait for statefulset to become available")
	Eventually(func() bool {
		if err := input.Getter.Get(ctx, key, input.StatefulSet); err == nil {
			if input.StatefulSet.Status.ObservedGeneration != input.StatefulSet.Generation {
				return false
			}

			return input.StatefulSet.Status.AvailableReplicas == input.StatefulSet.Status.Replicas
		}
		return false
	}, intervals...).Should(BeTrue())
	Logf("StatefulSet %s is now available, took %v", key, time.Since(start))
}
