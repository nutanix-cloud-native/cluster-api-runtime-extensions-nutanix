//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	addonsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

type WaitForCSIToBeReadyInWorkloadClusterInput struct {
	CSI                         *apivariables.CSI
	WorkloadCluster             *clusterv1.Cluster
	ClusterProxy                framework.ClusterProxy
	DeploymentIntervals         []interface{}
	DaemonSetIntervals          []interface{}
	HelmReleaseIntervals        []interface{}
	ClusterResourceSetIntervals []interface{}
}

func WaitForCSIToBeReadyInWorkloadCluster(
	ctx context.Context,
	input WaitForCSIToBeReadyInWorkloadClusterInput, //nolint:gocritic // This hugeParam is OK in tests.
) {
	if input.CSI == nil {
		return
	}

	for providerName, providerConfig := range input.CSI.Providers {
		switch providerName {
		case v1alpha1.CSIProviderLocalPath:
			waitForLocalPathCSIToBeReadyInWorkloadCluster(
				ctx,
				waitForLocalPathCSIToBeReadyInWorkloadClusterInput{
					strategy:                    providerConfig.Strategy,
					workloadCluster:             input.WorkloadCluster,
					clusterProxy:                input.ClusterProxy,
					deploymentIntervals:         input.DeploymentIntervals,
					helmReleaseIntervals:        input.HelmReleaseIntervals,
					clusterResourceSetIntervals: input.ClusterResourceSetIntervals,
				},
			)
		default:
			Fail(
				fmt.Sprintf(
					"Do not know how to wait for CSI provider %s to be ready",
					providerName,
				),
			)
		}

		waitForStorageClassesToExistInWorkloadCluster(
			ctx,
			waitForStorageClassesToExistInWorkloadClusterInput{
				storageClasses:  providerConfig.StorageClassConfigs,
				workloadCluster: input.WorkloadCluster,
				clusterProxy:    input.ClusterProxy,
				providerName:    providerName,
				defaultStorage:  input.CSI.DefaultStorage,
			},
		)
	}
}

type waitForLocalPathCSIToBeReadyInWorkloadClusterInput struct {
	strategy                    v1alpha1.AddonStrategy
	workloadCluster             *clusterv1.Cluster
	clusterProxy                framework.ClusterProxy
	deploymentIntervals         []interface{}
	helmReleaseIntervals        []interface{}
	clusterResourceSetIntervals []interface{}
}

func waitForLocalPathCSIToBeReadyInWorkloadCluster(
	ctx context.Context,
	input waitForLocalPathCSIToBeReadyInWorkloadClusterInput, //nolint:gocritic // This hugeParam is OK in tests.
) {
	switch input.strategy {
	case v1alpha1.AddonStrategyClusterResourceSet:
		crs := &addonsv1.ClusterResourceSet{}
		Expect(input.clusterProxy.GetClient().Get(
			ctx,
			types.NamespacedName{
				Name:      "local-path-provisioner-csi-" + input.workloadCluster.Name,
				Namespace: input.workloadCluster.Namespace,
			},
			crs,
		)).To(Succeed())

		framework.WaitForClusterResourceSetToApplyResources(
			ctx,
			framework.WaitForClusterResourceSetToApplyResourcesInput{
				ClusterResourceSet: crs,
				ClusterProxy:       input.clusterProxy,
				Cluster:            input.workloadCluster,
			},
			input.clusterResourceSetIntervals...,
		)
	case v1alpha1.AddonStrategyHelmAddon:
		WaitForHelmReleaseProxyReadyForCluster(
			ctx,
			WaitForHelmReleaseProxyReadyForClusterInput{
				GetLister:          input.clusterProxy.GetClient(),
				Cluster:            input.workloadCluster,
				HelmChartProxyName: "local-path-provisioner-csi-" + input.workloadCluster.Name,
			},
			input.helmReleaseIntervals...,
		)
	default:
		Fail(
			fmt.Sprintf(
				"Do not know how to wait for local-path-provisioner CSI using strategy %s to be ready",
				input.strategy,
			),
		)
	}

	workloadClusterClient := input.clusterProxy.GetWorkloadCluster(
		ctx, input.workloadCluster.Namespace, input.workloadCluster.Name,
	).GetClient()

	WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter: workloadClusterClient,
		Deployment: &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "local-path-provisioner",
				Namespace: metav1.NamespaceSystem,
			},
		},
	}, input.deploymentIntervals...)
}

type waitForStorageClassesToExistInWorkloadClusterInput struct {
	providerName    string
	storageClasses  map[string]v1alpha1.StorageClassConfig
	defaultStorage  v1alpha1.DefaultStorage
	workloadCluster *clusterv1.Cluster
	clusterProxy    framework.ClusterProxy
}

func waitForStorageClassesToExistInWorkloadCluster(
	ctx context.Context,
	input waitForStorageClassesToExistInWorkloadClusterInput, //nolint:gocritic // This hugeParam is OK in tests.
) {
	workloadClusterClient := input.clusterProxy.GetWorkloadCluster(
		ctx, input.workloadCluster.Namespace, input.workloadCluster.Name,
	).GetClient()

	var provisioner string
	switch input.providerName {
	case v1alpha1.CSIProviderLocalPath:
		provisioner = string(v1alpha1.LocalPathProvisioner)
	default:
		Fail(
			fmt.Sprintf(
				"Do not know how to wait for storage classes for CSI provider %s",
				input.providerName,
			),
		)
	}

	for storageClassName, storageClassConfig := range input.storageClasses {
		isDefault := input.providerName == input.defaultStorage.Provider &&
			storageClassName == input.defaultStorage.StorageClassConfig

		waitForStorageClassToExistInWorkloadCluster(
			ctx,
			workloadClusterClient,
			client.ObjectKey{Name: input.providerName + "-" + storageClassName},
			provisioner,
			storageClassConfig,
			isDefault,
		)
	}
}

func waitForStorageClassToExistInWorkloadCluster(
	ctx context.Context,
	workloadClusterClient client.Client,
	scKey client.ObjectKey,
	provisioner string,
	scConfig v1alpha1.StorageClassConfig,
	isDefault bool,
	intervals ...interface{},
) {
	start := time.Now()
	capie2e.Byf("waiting for storageclass %v to exist", scKey)
	Log("starting to wait for storageclass to exist")
	var gotSC storagev1.StorageClass
	Eventually(func() bool {
		if err := workloadClusterClient.Get(ctx, scKey, &gotSC); err != nil {
			if apierrors.IsNotFound(err) {
				return false
			}
			Expect(err).NotTo(HaveOccurred())
		}
		return true
	}, intervals...).Should(BeTrue())

	Expect(gotSC.Provisioner).To(Equal(provisioner))
	Expect(gotSC.Parameters).To(Equal(scConfig.Parameters))
	if scConfig.ReclaimPolicy != nil {
		Expect(gotSC.ReclaimPolicy).To(Equal(scConfig.ReclaimPolicy))
	} else {
		Expect(gotSC.ReclaimPolicy).To(BeNil())
	}
	if scConfig.VolumeBindingMode != nil {
		Expect(gotSC.VolumeBindingMode).To(Equal(scConfig.VolumeBindingMode))
	} else {
		Expect(gotSC.VolumeBindingMode).To(BeNil())
	}
	Expect(gotSC.AllowVolumeExpansion).To(HaveValue(Equal(scConfig.AllowExpansion)))
	if isDefault {
		Expect(gotSC.Annotations).To(HaveKeyWithValue("storageclass.kubernetes.io/is-default-class", "true"))
	} else {
		Expect(gotSC.Annotations).ToNot(HaveKeyWithValue("storageclass.kubernetes.io/is-default-class", "true"))
	}

	Logf("StorageClass %v now exists, took %v", scKey, time.Since(start))
}
