// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"
	"fmt"

	"github.com/blang/semver/v4"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	lifecycleutils "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/utils"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

type AWSCCMConfig struct {
	*options.GlobalOptions

	kubernetesMinorVersionToCCMConfigMapNames map[string]string
}

func (a *AWSCCMConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringToStringVar(
		&a.kubernetesMinorVersionToCCMConfigMapNames,
		prefix+".default-aws-ccm-configmap-names",
		map[string]string{
			"1.27": "aws-ccm-v1.27.1",
			"1.28": "aws-ccm-v1.28.1",
		},
		"map of provider cluster implementation type to default installation ConfigMap name",
	)
}

type AWSCCM struct {
	client ctrlclient.Client
	config *AWSCCMConfig
}

func New(
	c ctrlclient.Client,
	cfg *AWSCCMConfig,
) *AWSCCM {
	return &AWSCCM{
		client: c,
		config: cfg,
	}
}

func (a *AWSCCM) Apply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		cluster.Name,
	)
	log.Info("Creating AWS CCM ConfigMap for Cluster")
	version, err := semver.ParseTolerant(cluster.Spec.Topology.Version)
	if err != nil {
		return fmt.Errorf("failed to parse version from cluster %w", err)
	}
	minorVersion := fmt.Sprintf("%d.%d", version.Major, version.Minor)
	configMapForMinorVersion := a.config.kubernetesMinorVersionToCCMConfigMapNames[minorVersion]
	ccmConfigMapForMinorVersion := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: a.config.DefaultsNamespace(),
			Name:      configMapForMinorVersion,
		},
	}
	objName := ctrlclient.ObjectKeyFromObject(
		ccmConfigMapForMinorVersion,
	)
	err = a.client.Get(ctx, objName, ccmConfigMapForMinorVersion)
	if err != nil {
		log.Error(err, "failed to fetch CCM template for cluster")
		return fmt.Errorf(
			"failed to retrieve default AWS CCM manifests ConfigMap %q: %w",
			objName,
			err,
		)
	}

	ccmConfigMap := generateCCMConfigMapForCluster(ccmConfigMapForMinorVersion, cluster)
	if err = client.ServerSideApply(ctx, a.client, ccmConfigMap); err != nil {
		log.Error(err, "failed to apply CCM configmap for cluster")
		return fmt.Errorf(
			"failed to apply AWS CCM manifests ConfigMap: %w",
			err,
		)
	}

	err = lifecycleutils.EnsureCRSForClusterFromObjects(
		ctx,
		ccmConfigMap.Name,
		a.client,
		cluster,
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
			Name:      fmt.Sprintf("%s-%s", ccmConfigMapForVersion.Name, cluster.Name),
		},
		Data: ccmConfigMapForVersion.Data,
	}
	return ccmConfigMapForCluster
}
