//go:build e2e

// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"time"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WaitForDeploymentsAvailable waits until the Deployment has observedGeneration equal to generation and
// status.Available = True, that signals that all the desired replicas are in place.
func WaitForDeploymentsAvailable(
	ctx context.Context, input framework.WaitForDeploymentsAvailableInput, intervals ...interface{},
) {
	start := time.Now()
	namespace, name := input.Deployment.GetNamespace(), input.Deployment.GetName()
	Byf("waiting for deployment %s/%s to be available", namespace, name)
	Log("starting to wait for deployment to become available")
	Eventually(func() bool {
		key := client.ObjectKey{Namespace: namespace, Name: name}
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
	Logf("Deployment %s/%s is now available, took %v", namespace, name, time.Since(start))
}
