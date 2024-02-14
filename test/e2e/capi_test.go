//go:build e2e

// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/test/framework"
)

var _ = Describe("Running the Cluster API E2E tests", func() {
	AfterEach(func() {
		CheckTestBeforeCleanup()
	})

	Context("Check providers are running", func() {
		It("List running pods", func() {
			deployments, err := bootstrapClusterProxy.GetClientSet().
				AppsV1().
				Deployments(corev1.NamespaceAll).
				List(context.Background(), v1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())

			readyReplicasMap := make(map[string]int32, len(deployments.Items))
			for _, d := range deployments.Items {
				readyReplicasMap[d.Namespace+"/"+d.Name] = d.Status.ReadyReplicas
			}
			GinkgoWriter.Println(framework.PrettyPrint(readyReplicasMap))
		})
	})
})
