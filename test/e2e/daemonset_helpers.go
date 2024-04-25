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

// WaitForDaemonSetsAvailableInput is the input for WaitForDaemonSetsAvailable.
type WaitForDaemonSetsAvailableInput struct {
	Getter    framework.Getter
	DaemonSet *appsv1.DaemonSet
}

// WaitForDaemonSetsAvailable waits until the DaemonSet has observedGeneration equal to generation and
// status.Available = True, that signals that all the desired replicas are in place.
func WaitForDaemonSetsAvailable(
	ctx context.Context, input WaitForDaemonSetsAvailableInput, intervals ...interface{},
) {
	start := time.Now()
	namespace, name := input.DaemonSet.GetNamespace(), input.DaemonSet.GetName()
	capie2e.Byf("waiting for deployment %s/%s to be available", namespace, name)
	Log("starting to wait for deployment to become available")
	Eventually(func() bool {
		key := client.ObjectKey{Namespace: namespace, Name: name}
		if err := input.Getter.Get(ctx, key, input.DaemonSet); err == nil {
			if input.DaemonSet.Status.ObservedGeneration != input.DaemonSet.Generation {
				return false
			}

			if input.DaemonSet.Status.NumberAvailable != input.DaemonSet.Status.DesiredNumberScheduled {
				return false
			}

			return input.DaemonSet.Status.NumberUnavailable == 0
		}
		return false
	}, intervals...).Should(BeTrue())
	Logf("DaemonSet %s/%s is now available, took %v", namespace, name, time.Since(start))
}
