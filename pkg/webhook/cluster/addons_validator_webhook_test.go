// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

func TestBlockWithNonExistentHelmChartConfig(t *testing.T) {
	g := NewWithT(t)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-with-non-existent-helm-chart-configmap",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: clusterv1.ClusterSpec{
			Topology: &clusterv1.Topology{
				Variables: []clusterv1.ClusterVariable{
					*helmChartConfigVariable(t, "non-existent-helm-chart-configmap"),
				},
			},
		},
	}
	err := env.Client.Create(ctx, cluster)
	g.Expect(err).To(HaveOccurred())
	g.Expect(strings.Contains(
		err.Error(),
		"HelmChart ConfigMap \"non-existent-helm-chart-configmap\" referenced in the cluster variables not found",
	)).
		To(BeTrue(), "Expected error to be of type IsNotFound")
}

func TestAllowWithExistingHelmChartConfig(t *testing.T) {
	g := NewWithT(t)

	configmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "custom-helm-chart-configmap",
			Namespace: metav1.NamespaceDefault,
		},
		Data: map[string]string{
			"ccm": "test chart config data",
		},
	}
	g.Expect(env.Client.Create(ctx, configmap)).ToNot(HaveOccurred())

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "custom-helm-chart-configmap",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: clusterv1.ClusterSpec{
			Topology: &clusterv1.Topology{
				Variables: []clusterv1.ClusterVariable{
					*helmChartConfigVariable(t, "custom-helm-chart-configmap"),
				},
			},
		},
	}
	g.Expect(env.Client.Create(ctx, cluster)).ToNot(HaveOccurred())
}

func helmChartConfigVariable(
	t *testing.T,
	name string,
) *clusterv1.ClusterVariable {
	t.Helper()
	hv, err := apivariables.MarshalToClusterVariable(
		"clusterConfig",
		&apivariables.ClusterConfigSpec{
			Addons: &apivariables.Addons{
				GenericAddons: v1alpha1.GenericAddons{
					HelmChartConfig: &v1alpha1.HelmChartConfig{
						ConfigMapRef: v1alpha1.LocalObjectReference{
							Name: name,
						},
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("failed to create addon variable: %s", err)
	}
	return hv
}
