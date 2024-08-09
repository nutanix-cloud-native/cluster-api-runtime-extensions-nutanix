//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	. "github.com/onsi/gomega"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	helmaddonsv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/addons"
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

	hcp, err := getHelmChartProxy(
		ctx,
		input.GetLister,
		input.Cluster.Name,
		input.Cluster.Namespace,
		input.HelmReleaseName,
	)
	Expect(err).ToNot(HaveOccurred())

	hrp, err := getHelmReleaseProxy(
		ctx,
		input.GetLister,
		input.Cluster.Name,
		input.Cluster.Namespace,
		hcp.Name,
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

func getHelmChartProxy(
	ctx context.Context,
	getLister framework.GetLister,
	clusterName string,
	clusterNamespace string,
	helmReleaseName string,
) (*helmaddonsv1.HelmChartProxy, error) {
	// Get the HelmChartProxy using label selectors since we don't know the name of the HelmChartProxy.
	chartList := &helmaddonsv1.HelmChartProxyList{}
	labels := map[string]string{
		addons.ClusterNamespaceLabel:    clusterNamespace,
		clusterv1.ClusterNameLabel:      clusterName,
		// Reflects the hash implementation used by the helm addon strategy.
		addons.HelmReleaseNameHashLabel: fmt.Sprintf("%x", sha256.Sum224([]byte(helmReleaseName))),
	}
	if err := getLister.List(
		ctx,
		chartList,
		ctrlclient.InNamespace(clusterNamespace),
		ctrlclient.MatchingLabels(labels),
	); err != nil {
		return nil, err
	}

	if len(chartList.Items) != 1 {
		return nil, fmt.Errorf("expected 1 HelmChartProxy, got %d", len(chartList.Items))
	}

	return &chartList.Items[0], nil
}
