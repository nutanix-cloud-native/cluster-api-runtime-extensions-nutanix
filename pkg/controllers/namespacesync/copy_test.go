// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package namespacesync

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/storage/names"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDoNotUpdateIfTargetExists(t *testing.T) {
	g := NewWithT(t)
	timeout := 50 * time.Second

	prefix := names.SimpleNameGenerator.GenerateName("test-")
	sourceClusterClassName, cleanup, err := createClusterClassAndTemplates(
		prefix,
		sourceClusterClassNamespace,
	)
	g.Expect(err).ToNot(HaveOccurred())

	targetNamespaces, err := createTargetNamespaces(3)
	g.Expect(err).ToNot(HaveOccurred())

	for _, targetNamespace := range targetNamespaces {
		g.Eventually(func() error {
			return verifyClusterClassAndTemplates(
				env.Client,
				sourceClusterClassName,
				targetNamespace.Name,
			)
		},
			timeout,
		).Should(Succeed())
	}

	// Delete source class
	g.Expect(cleanup()).To(Succeed())

	// Create source class again
	sourceClusterClassName, cleanup, err = createClusterClassAndTemplates(
		prefix,
		sourceClusterClassNamespace,
	)
	g.Expect(err).ToNot(HaveOccurred())
	defer func() {
		g.Expect(cleanup()).To(Succeed())
	}()

	source := &clusterv1.ClusterClass{}
	err = env.Get(
		ctx,
		client.ObjectKey{
			Namespace: sourceClusterClassNamespace,
			Name:      sourceClusterClassName,
		},
		source,
	)
	g.Expect(err).ToNot(HaveOccurred())

	// Verify that the copy function returns an error because the target ClusterClass and Templates
	// already exist.
	for _, targetNamespace := range targetNamespaces {
		err = copyClusterClassAndTemplates(
			ctx,
			env.Client,
			env.Client,
			source,
			targetNamespace.Name,
		)
		g.Expect(apierrors.IsAlreadyExists(err)).To(BeTrue())
	}
}
