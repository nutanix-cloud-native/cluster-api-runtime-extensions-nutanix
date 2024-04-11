//go:build e2e

// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/gomega"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	addonsv1 "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
)

// WaitForHelmReleaseProxyReadyInput is the input for WaitForHelmReleaseProxyReady.
type WaitForHelmReleaseProxyReadyForClusterInput struct {
	GetLister          framework.GetLister
	Cluster            *clusterv1.Cluster
	HelmChartProxyName string
}

// WaitForHelmReleaseProxyReady waits until the HelmReleaseProxy has ready condition = True, that signals that the Helm
// install was successful.
func WaitForHelmReleaseProxyReadyForCluster(
	ctx context.Context,
	input WaitForHelmReleaseProxyReadyForClusterInput,
	intervals ...interface{},
) {
	start := time.Now()
	hrp, err := getHelmReleaseProxy(
		ctx,
		input.GetLister,
		input.Cluster.Name,
		input.Cluster.Namespace,
		input.HelmChartProxyName,
	)
	Expect(err).ToNot(HaveOccurred())
	hrpKey := ctrlclient.ObjectKeyFromObject(hrp)

	capie2e.Byf("waiting for HelmReleaseProxy for %s to be ready", hrpKey)
	Log("starting to wait for HelmReleaseProxy to become available")
	Eventually(func() bool {
		err := input.GetLister.Get(ctx, hrpKey, hrp)

		return err == nil && conditions.IsTrue(hrp, clusterv1.ReadyCondition)
	}, intervals...).Should(
		BeTrue(),
		fmt.Sprintf("HelmReleaseProxy %s failed to become ready and have up to date revision: ready condition = %+v, "+
			"revision = %v, full object is:\n%+v\n`",
			hrpKey, conditions.Get(hrp, clusterv1.ReadyCondition), hrp.Status.Revision, hrp),
	)
	Logf("HelmReleaseProxy %s is now ready, took %v", hrpKey, time.Since(start))
}

func getHelmReleaseProxy(
	ctx context.Context,
	getLister framework.GetLister,
	clusterName string,
	clusterNamespace string,
	helmChartProxyName string,
) (*addonsv1.HelmReleaseProxy, error) {
	// Get the HelmReleaseProxy using label selectors since we don't know the name of the HelmReleaseProxy.
	releaseList := &addonsv1.HelmReleaseProxyList{}
	labels := map[string]string{
		clusterv1.ClusterNameLabel:       clusterName,
		addonsv1.HelmChartProxyLabelName: helmChartProxyName,
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
