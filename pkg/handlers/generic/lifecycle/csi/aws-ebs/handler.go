// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"
	"fmt"

	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	lifecycleutils "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/utils"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

type AWSEBSConfig struct {
	*options.GlobalOptions
	defaultAWSEBSConfigMapName string
}

func (a *AWSEBSConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&a.defaultAWSEBSConfigMapName,
		prefix+".aws-ebs-provider-configmap-name",
		"aws-ebs-csi",
		"name of the ConfigMap used to deploy AWS EBS CSI driver",
	)
}

type AWSEBS struct {
	client ctrlclient.Client
	config *AWSEBSConfig
}

func New(
	c ctrlclient.Client,
	cfg *AWSEBSConfig,
) *AWSEBS {
	return &AWSEBS{
		client: c,
		config: cfg,
	}
}

func (a *AWSEBS) Apply(
	ctx context.Context,
	provider v1alpha1.CSIProvider,
	defaultStorageConfig *v1alpha1.DefaultStorage,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
) error {
	strategy := provider.Strategy
	switch strategy {
	case v1alpha1.AddonStrategyClusterResourceSet:
		err := a.handleCRSApply(ctx, req)
		if err != nil {
			return err
		}
	case v1alpha1.AddonStrategyHelmAddon:
	default:
		return fmt.Errorf("stategy %s not implemented", strategy)
	}
	return a.createStorageClasses(
		ctx,
		provider.StorageClassConfig,
		&req.Cluster,
		defaultStorageConfig,
	)
}

func (a *AWSEBS) createStorageClasses(ctx context.Context,
	configs []v1alpha1.StorageClassConfig,
	cluster *clusterv1.Cluster,
	defaultStorageConfig *v1alpha1.DefaultStorage,
) error {
	for _, c := range configs {
		setAsDefault := c.Name == defaultStorageConfig.StorageClassConfigName &&
			v1alpha1.CSIProviderAWSEBS == defaultStorageConfig.ProviderName
		err := lifecycleutils.CreateStorageClass(
			ctx,
			a.client,
			c,
			cluster,
			a.config.GlobalOptions.DefaultsNamespace(),
			setAsDefault,
		)
		if err != nil {
			return fmt.Errorf("failed to create storageclass %w", err)
		}
	}
	return nil
}

func (a *AWSEBS) handleCRSApply(ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
) error {
	awsEBSCSIConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: a.config.DefaultsNamespace(),
			Name:      a.config.defaultAWSEBSConfigMapName,
		},
	}
	defaultAWSEBSCSIConfigMapObjName := ctrlclient.ObjectKeyFromObject(
		awsEBSCSIConfigMap,
	)
	err := a.client.Get(ctx, defaultAWSEBSCSIConfigMapObjName, awsEBSCSIConfigMap)
	if err != nil {
		return fmt.Errorf(
			"failed to retrieve default AWS EBS CSI manifests ConfigMap %q: %w",
			defaultAWSEBSCSIConfigMapObjName,
			err,
		)
	}
	cluster := req.Cluster
	cm := generateAWSEBSCSIConfigMap(awsEBSCSIConfigMap, &cluster)
	if err := client.ServerSideApply(ctx, a.client, cm); err != nil {
		return fmt.Errorf(
			"failed to apply AWS EBS CSI manifests ConfigMap: %w",
			err,
		)
	}
	err = lifecycleutils.EnsureCRSForClusterFromConfigMaps(
		ctx,
		cm.Name,
		a.client,
		&req.Cluster,
		cm,
	)
	if err != nil {
		return err
	}
	return nil
}

func generateAWSEBSCSIConfigMap(
	defaultAWSEBSCSIConfigMap *corev1.ConfigMap, cluster *clusterv1.Cluster,
) *corev1.ConfigMap {
	namespacedAWSEBSConfigMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      fmt.Sprintf("%s-%s", defaultAWSEBSCSIConfigMap.Name, cluster.Name),
		},
		Data:       defaultAWSEBSCSIConfigMap.Data,
		BinaryData: defaultAWSEBSCSIConfigMap.BinaryData,
	}

	return namespacedAWSEBSConfigMap
}
