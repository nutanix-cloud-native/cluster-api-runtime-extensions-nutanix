// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"
	"fmt"

	"github.com/blang/semver/v4"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

type crsConfig struct {
	kubernetesMinorVersionToAWSCCMVersion map[string]string
}

type crsStrategy struct {
	config crsConfig

	client ctrlclient.Client
}

func (s crsStrategy) Apply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	defaultsNamespace string,
	log logr.Logger,
) error {
	log.Info("Creating AWS CCM ConfigMap for Cluster")
	version, err := semver.ParseTolerant(cluster.Spec.Topology.Version)
	if err != nil {
		return fmt.Errorf("failed to parse version from cluster: %w", err)
	}
	minorVersion := fmt.Sprintf("%d.%d", version.Major, version.Minor)
	ccmVersionForK8sMinorVersion := s.config.kubernetesMinorVersionToAWSCCMVersion[minorVersion]
	ccmConfigMapForMinorVersion := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: defaultsNamespace,
			Name:      awsCCMPrefix + ccmVersionForK8sMinorVersion,
		},
	}
	objName := ctrlclient.ObjectKeyFromObject(
		ccmConfigMapForMinorVersion,
	)
	err = s.client.Get(ctx, objName, ccmConfigMapForMinorVersion)
	if err != nil {
		log.Error(err, "failed to fetch CCM template for cluster")
		return fmt.Errorf(
			"failed to retrieve default AWS CCM manifests ConfigMap %q: %w",
			objName,
			err,
		)
	}

	ccmConfigMap := generateCCMConfigMapForCluster(ccmConfigMapForMinorVersion, cluster)
	if err = client.ServerSideApply(ctx, s.client, ccmConfigMap, client.ForceOwnership); err != nil {
		log.Error(err, "failed to apply CCM configmap for cluster")
		return fmt.Errorf(
			"failed to apply AWS CCM manifests ConfigMap: %w",
			err,
		)
	}

	err = handlersutils.EnsureCRSForClusterFromObjects(
		ctx,
		ccmConfigMap.Name,
		s.client,
		cluster,
		handlersutils.DefaultEnsureCRSForClusterFromObjectsOptions(),
		ccmConfigMap,
	)
	if err != nil {
		return fmt.Errorf("failed to generate CCM CRS for cluster: %w", err)
	}

	return nil
}

func generateCCMConfigMapForCluster(
	ccmConfigMapForVersion *corev1.ConfigMap, cluster *clusterv1.Cluster,
) *corev1.ConfigMap {
	ccmConfigMapForCluster := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      awsCCMPrefix + cluster.Name,
		},
		Data: ccmConfigMapForVersion.Data,
	}
	return ccmConfigMapForCluster
}
