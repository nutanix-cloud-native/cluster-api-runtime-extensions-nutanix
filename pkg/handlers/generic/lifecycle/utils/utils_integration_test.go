// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var _ = Describe("Namespace", func() {
	It("creates a new namespace", func(ctx SpecContext) {
		c, err := helpers.TestEnv.GetK8sClient()
		Expect(err).To(BeNil())

		namespaceName := "new"

		Expect(EnsureNamespace(ctx, c, namespaceName)).To(Succeed())
		Expect(c.Delete(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceName,
			},
		})).To((Succeed()))
	})

	It("updates a namespace, preserving user-managed fields", func(ctx SpecContext) {
		c, err := helpers.TestEnv.GetK8sClient()
		Expect(err).To(BeNil())

		namespaceName := "existing"
		Expect(c.Create(ctx,
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
					Labels: map[string]string{
						"userkey": "uservalue",
					},
				},
			})).To(Succeed())

		Expect(EnsureNamespace(ctx, c, namespaceName)).To(Succeed())

		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceName,
			},
		}
		Expect(c.Get(ctx, client.ObjectKeyFromObject(ns), ns)).To(Succeed())
		Expect(ns.GetLabels()).To(HaveKeyWithValue("userkey", "uservalue"))

		Expect(c.Delete(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceName,
			},
		})).To((Succeed()))
	})
})
