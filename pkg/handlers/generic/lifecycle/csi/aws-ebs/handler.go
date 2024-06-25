// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	csiutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/csi/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

var defaultStorageClassParameters = map[string]string{
	"csi.storage.k8s.io/fstype": "ext4",
	"type":                      "gp3",
}

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
	defaultStorage v1alpha1.DefaultStorage,
	cluster *clusterv1.Cluster,
	_ logr.Logger,
) error {
	strategy := provider.Strategy
	switch strategy {
	case v1alpha1.AddonStrategyClusterResourceSet:
		err := a.handleCRSApply(ctx, cluster)
		if err != nil {
			return err
		}
	case v1alpha1.AddonStrategyHelmAddon:
	default:
		return fmt.Errorf("stategy %s not implemented", strategy)
	}
	err := csiutils.CreateStorageClassesOnRemote(
		ctx,
		a.client,
		provider.StorageClassConfigs,
		cluster,
		defaultStorage,
		v1alpha1.CSIProviderAWSEBS,
		v1alpha1.AWSEBSProvisioner,
		defaultStorageClassParameters,
	)
	if err != nil {
		return fmt.Errorf("error creating StorageClasses for the AWS EBS CSI driver: %w", err)
	}
	return nil
}

func (a *AWSEBS) handleCRSApply(ctx context.Context,
	cluster *clusterv1.Cluster,
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
	cm := generateAWSEBSCSIConfigMap(awsEBSCSIConfigMap, cluster)
	if err := client.ServerSideApply(ctx, a.client, cm, client.ForceOwnership); err != nil {
		return fmt.Errorf(
			"failed to apply AWS EBS CSI manifests ConfigMap: %w",
			err,
		)
	}
	err = handlersutils.EnsureCRSForClusterFromObjects(
		ctx,
		cm.Name,
		a.client,
		cluster,
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
