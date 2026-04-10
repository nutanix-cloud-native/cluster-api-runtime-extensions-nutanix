//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/gomega"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	helmaddonsv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

// WaitForHelmReleaseProxyReadyInput is the input for WaitForHelmReleaseProxyReady.
type WaitForHelmReleaseProxyReadyForClusterInput struct {
	GetLister       framework.GetLister
	Cluster         *clusterv1.Cluster
	HelmReleaseName string
}

// WaitForHelmReleaseProxyReady waits until the HelmReleaseProxy has ready condition = True, that signals that the Helm
// install was successful.
func WaitForHelmReleaseProxyReadyForCluster(
	ctx context.Context,
	input WaitForHelmReleaseProxyReadyForClusterInput,
	intervals ...interface{},
) {
	start := time.Now()

	var (
		hrp         *helmaddonsv1.HelmReleaseProxy
		hrpKey      ctrlclient.ObjectKey
		lastListErr error
	)

	capie2e.Byf(
		"waiting for HelmReleaseProxy %q to be ready for cluster %s/%s",
		input.HelmReleaseName,
		input.Cluster.Namespace,
		input.Cluster.Name,
	)
	Log("starting to wait for HelmReleaseProxy to be created and become ready")
	Eventually(func() bool {
		// Re-read the Cluster each time to ensure we see webhook-assigned annotations.
		currentCluster := &clusterv1.Cluster{}
		if err := input.GetLister.Get(
			ctx,
			ctrlclient.ObjectKey{Namespace: input.Cluster.Namespace, Name: input.Cluster.Name},
			currentCluster,
		); err != nil {
			lastListErr = err
			return false
		}

		clusterUUID := currentCluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey]
		if clusterUUID == "" {
			// Cluster UUID is assigned by webhook; the HRP label depends on it.
			return false
		}

		var err error
		hrp, err = getHelmReleaseProxy(
			ctx,
			input.GetLister,
			input.Cluster.Name,
			input.Cluster.Namespace,
			fmt.Sprintf("%s-%s", input.HelmReleaseName, clusterUUID),
		)
		if err != nil {
			lastListErr = err
			return false
		}
		hrpKey = ctrlclient.ObjectKeyFromObject(hrp)

		err = input.GetLister.Get(ctx, hrpKey, hrp)
		if err != nil {
			lastListErr = err
			return false
		}

		lastListErr = nil
		return apimeta.IsStatusConditionTrue(hrp.GetConditions(), clusterv1.ReadyCondition)
	}, intervals...).Should(
		BeTrue(),
		fmt.Sprintf(
			"HelmReleaseProxy for %s/%s (%q) failed to become ready.\n"+
				"last error = %v\n"+
				"last ready condition = %+v\n"+
				"revision = %v\n"+
				"object =\n%+v\n",
			input.Cluster.Namespace,
			input.Cluster.Name,
			input.HelmReleaseName,
			lastListErr,
			func() any {
				if hrp == nil {
					return nil
				}
				return apimeta.FindStatusCondition(hrp.GetConditions(), clusterv1.ReadyCondition)
			}(),
			func() any {
				if hrp == nil {
					return nil
				}
				return hrp.Status.Revision
			}(),
			func() any {
				if hrp == nil {
					return nil
				}
				return hrp
			}(),
		),
	)
	Logf("HelmReleaseProxy %s is now ready, took %v", hrpKey, time.Since(start))
}

func getHelmReleaseProxy(
	ctx context.Context,
	getLister framework.GetLister,
	clusterName string,
	clusterNamespace string,
	helmChartProxyName string,
) (*helmaddonsv1.HelmReleaseProxy, error) {
	// Get the HelmReleaseProxy using label selectors since we don't know the name of the HelmReleaseProxy.
	releaseList := &helmaddonsv1.HelmReleaseProxyList{}
	labels := map[string]string{
		clusterv1.ClusterNameLabel:           clusterName,
		helmaddonsv1.HelmChartProxyLabelName: helmChartProxyName,
	}
	if err := getLister.List(
		ctx,
		releaseList,
		ctrlclient.InNamespace(clusterNamespace),
		ctrlclient.MatchingLabels(labels),
	); err != nil {
		return nil, err
	}

	if len(releaseList.Items) != 1 {
		return nil, fmt.Errorf("expected 1 HelmReleaseProxy, got %d", len(releaseList.Items))
	}

	return &releaseList.Items[0], nil
}
