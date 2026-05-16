// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var _ = Describe("Namespace", func() {
	It("creates a new namespace", func(ctx SpecContext) {
		c, err := helpers.TestEnv.GetK8sClient()
		Expect(err).To(BeNil())

		namespaceName := names.SimpleNameGenerator.GenerateName("test-")

		Expect(EnsureNamespaceWithName(ctx, c, namespaceName)).To(Succeed())
		Expect(c.Delete(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceName,
			},
		})).To((Succeed()))
	})

	It(
		"updates a namespace with no changes, preserving user-managed fields",
		func(ctx SpecContext) {
			c, err := helpers.TestEnv.GetK8sClient()
			Expect(err).To(BeNil())

			namespaceName := names.SimpleNameGenerator.GenerateName("test-")
			Expect(c.Create(ctx,
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: namespaceName,
						Labels: map[string]string{
							"userkey": "uservalue",
						},
					},
				})).To(Succeed())

			Expect(EnsureNamespaceWithName(ctx, c, namespaceName)).To(Succeed())

			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
				},
			}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(ns), ns)).To(Succeed())
			Expect(ns.GetLabels()).To(HaveKeyWithValue("userkey", "uservalue"))
		},
	)

	It(
		"updates a namespace by only sending new labels, preserving existing user-managed fields",
		func(ctx SpecContext) {
			c, err := helpers.TestEnv.GetK8sClient()
			Expect(err).To(BeNil())

			namespaceName := names.SimpleNameGenerator.GenerateName("test-")
			Expect(c.Create(ctx,
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: namespaceName,
						Labels: map[string]string{
							"userkey": "uservalue",
						},
					},
				})).To(Succeed())

			Expect(EnsureNamespace(ctx, c, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
					Labels: map[string]string{
						"newkey": "newvalue",
					},
				},
			})).To(Succeed())

			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
				},
			}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(ns), ns)).To(Succeed())
			Expect(ns.GetLabels()).To(HaveKeyWithValue("userkey", "uservalue"))
			Expect(ns.GetLabels()).To(HaveKeyWithValue("newkey", "newvalue"))
		},
	)

	It(
		"applies the privileged PSA enforce label without an enforce-version label, idempotently, "+
			"and on a pre-existing label-less namespace",
		func(ctx SpecContext) {
			c, err := helpers.TestEnv.GetK8sClient()
			Expect(err).To(BeNil())

			// Case 1: brand-new namespace gets the label.
			newName := names.SimpleNameGenerator.GenerateName("psa-new-")
			Expect(EnsureNamespaceWithMetadata(
				ctx, c, newName, PrivilegedPodSecurityEnforceLabels, nil,
			)).To(Succeed())

			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: newName}}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(ns), ns)).To(Succeed())
			Expect(ns.GetLabels()).To(HaveKeyWithValue(
				"pod-security.kubernetes.io/enforce", "privileged",
			))
			Expect(ns.GetLabels()).NotTo(HaveKey(
				"pod-security.kubernetes.io/enforce-version",
			))

			// Case 2: calling again is a no-op (idempotent).
			Expect(EnsureNamespaceWithMetadata(
				ctx, c, newName, PrivilegedPodSecurityEnforceLabels, nil,
			)).To(Succeed())

			ns2 := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: newName}}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(ns2), ns2)).To(Succeed())
			Expect(ns2.GetLabels()).To(HaveKeyWithValue(
				"pod-security.kubernetes.io/enforce", "privileged",
			))

			// Case 3: a pre-existing namespace without labels is updated in place.
			preName := names.SimpleNameGenerator.GenerateName("psa-pre-")
			Expect(c.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: preName},
			})).To(Succeed())

			Expect(EnsureNamespaceWithMetadata(
				ctx, c, preName, PrivilegedPodSecurityEnforceLabels, nil,
			)).To(Succeed())

			pre := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: preName}}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(pre), pre)).To(Succeed())
			Expect(pre.GetLabels()).To(HaveKeyWithValue(
				"pod-security.kubernetes.io/enforce", "privileged",
			))
		},
	)
})
