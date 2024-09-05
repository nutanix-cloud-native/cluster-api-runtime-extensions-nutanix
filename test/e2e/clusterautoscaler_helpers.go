//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	addonsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/yaml"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
)

const clusterAutoscalerReleaseName = "ca"

type WaitForClusterAutoscalerToBeReadyForWorkloadClusterInput struct {
	ClusterAutoscaler           *v1alpha1.ClusterAutoscaler
	WorkloadCluster             *clusterv1.Cluster
	ClusterProxy                framework.ClusterProxy
	DeploymentIntervals         []interface{}
	DaemonSetIntervals          []interface{}
	HelmReleaseIntervals        []interface{}
	ClusterResourceSetIntervals []interface{}
}

func WaitForClusterAutoscalerToBeReadyForWorkloadCluster(
	ctx context.Context,
	input WaitForClusterAutoscalerToBeReadyForWorkloadClusterInput, //nolint:gocritic // This hugeParam is OK in tests.
) {
	if input.ClusterAutoscaler == nil {
		return
	}

	workloadClusterClient := input.ClusterProxy.GetWorkloadCluster(
		ctx, input.WorkloadCluster.Namespace, input.WorkloadCluster.Name,
	).GetClient()
	// Only check for ClusterAutoscaler if the cluster is self-managed.
	// managementCluster will be nil if workloadClusterClient is not a self-managed cluster.
	managementCluster, err := utils.ManagementCluster(ctx, workloadClusterClient)
	Expect(err).NotTo(HaveOccurred())
	if managementCluster == nil {
		return
	}

	switch ptr.Deref(input.ClusterAutoscaler.Strategy, "") {
	case v1alpha1.AddonStrategyClusterResourceSet:
		crs := &addonsv1.ClusterResourceSet{}
		Expect(input.ClusterProxy.GetClient().Get(
			ctx,
			types.NamespacedName{
				Name: fmt.Sprintf(
					"%s-%s",
					clusterAutoscalerReleaseName,
					input.WorkloadCluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey],
				),
				Namespace: input.WorkloadCluster.Namespace,
			},
			crs,
		)).To(Succeed())

		framework.WaitForClusterResourceSetToApplyResources(
			ctx,
			framework.WaitForClusterResourceSetToApplyResourcesInput{
				ClusterResourceSet: crs,
				ClusterProxy:       input.ClusterProxy,
				Cluster:            input.WorkloadCluster,
			},
			input.ClusterResourceSetIntervals...,
		)
	case v1alpha1.AddonStrategyHelmAddon:
		WaitForHelmReleaseProxyReadyForCluster(
			ctx,
			WaitForHelmReleaseProxyReadyForClusterInput{
				GetLister:       input.ClusterProxy.GetClient(),
				Cluster:         input.WorkloadCluster,
				HelmReleaseName: clusterAutoscalerReleaseName,
			},
			input.HelmReleaseIntervals...,
		)
	case "":
		Fail("Strategy not provided for cluster autoscaler")
	default:
		Fail(
			fmt.Sprintf(
				"Do not know how to wait for cluster autoscaler using strategy %s to be ready",
				*input.ClusterAutoscaler.Strategy,
			),
		)
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: input.WorkloadCluster.Namespace,
			Name: fmt.Sprintf(
				"cluster-autoscaler-%s",
				input.WorkloadCluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey],
			),
		},
	}

	WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter:     workloadClusterClient,
		Deployment: deployment,
	}, input.DeploymentIntervals...)

	Expect(deployment.Spec.Selector.MatchLabels).NotTo(BeEmpty())
	Expect(deployment.Spec.Selector.MatchLabels).To(
		HaveKeyWithValue(
			"app.kubernetes.io/instance",
			SatisfyAll(
				HavePrefix("ca-"),
				WithTransform(
					func(s string) error { return uuid.Validate(strings.TrimPrefix(s, "ca-")) },
					Not(HaveOccurred()),
				),
			),
		),
	)

	statusConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem,
			Name:      "cluster-autoscaler-status",
		},
	}

	WaitForConfigMapData(ctx, WaitForConfigMapDataInput{
		Getter:    workloadClusterClient,
		ConfigMap: statusConfigMap,
		DataValidator: func(data map[string]string) bool {
			type clusterAutoscalerStatus struct {
				AutoScalerStatus string `json:"autoscalerStatus,omitempty" yaml:"autoscalerStatus,omitempty"`
			}
			status := &clusterAutoscalerStatus{}
			err = yaml.Unmarshal([]byte(data["status"]), status)
			return err == nil && status.AutoScalerStatus == "Running"
		},
	}, input.DeploymentIntervals...)
}
