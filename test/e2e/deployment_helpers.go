//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"time"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WaitForDeploymentsAvailable waits until the Deployment has observedGeneration equal to generation and
// status.Available = True, that signals that all the desired replicas are in place.
func WaitForDeploymentsAvailable(
	ctx context.Context, input framework.WaitForDeploymentsAvailableInput, intervals ...interface{},
) {
	start := time.Now()
	key := client.ObjectKeyFromObject(input.Deployment)
	capie2e.Byf("waiting for deployment %s to be available", key)
	Log("starting to wait for deployment to become available")
	Eventually(func() bool {
		if err := input.Getter.Get(ctx, key, input.Deployment); err == nil {
			if input.Deployment.Status.ObservedGeneration != input.Deployment.Generation {
				return false
			}
			for _, c := range input.Deployment.Status.Conditions {
				if c.Type == appsv1.DeploymentAvailable && c.Status == corev1.ConditionTrue {
					return true
				}
			}
		}
		return false
	}, intervals...).Should(BeTrue(), func() string {
		return framework.DescribeFailedDeployment(input, input.Deployment)
	})
	Logf("Deployment %s is now available, took %v", key, time.Since(start))
}
