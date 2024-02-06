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

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/k8s/client"
)

type AWSCPIConfig struct {
	defaultsNamespace string

	kubernetesMinorVersionToCPIConfigMapNames map[string]string
}

func (a *AWSCPIConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&a.defaultsNamespace,
		prefix+".defaults-namespace",
		corev1.NamespaceDefault,
		"namespace of the ConfigMap used to deploy AWS CPI",
	)
	flags.StringToStringVar(
		&a.kubernetesMinorVersionToCPIConfigMapNames,
		prefix+".default-aws-cpi-configmap-names",
		map[string]string{
			"1.27": "aws-cpi-v1.27.1",
		},
		"map of provider cluster implementation type to default installation ConfigMap name",
	)
}

type AWSCPI struct {
	client ctrlclient.Client
	config *AWSCPIConfig
}

func New(
	c ctrlclient.Client,
	cfg *AWSCPIConfig,
) *AWSCPI {
	return &AWSCPI{
		client: c,
		config: cfg,
	}
}

func (a *AWSCPI) EnsureCPIConfigMapForCluster(
	ctx context.Context,
	cluster *clusterv1.Cluster,
) (*corev1.ConfigMap, error) {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		cluster.Name,
	)
	log.Info("Creating AWS CPI ConfigMap for Cluster")
	version, err := semver.ParseTolerant(cluster.Spec.Topology.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to parse version from cluster %w", err)
	}
	minorVersion := fmt.Sprintf("%d.%d", version.Major, version.Minor)
	configMapForMinorVersion := a.config.kubernetesMinorVersionToCPIConfigMapNames[minorVersion]
	cpiConfigMapForMinorVersion := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: a.config.defaultsNamespace,
			Name:      configMapForMinorVersion,
		},
	}
	objName := ctrlclient.ObjectKeyFromObject(
		cpiConfigMapForMinorVersion,
	)
	err = a.client.Get(ctx, objName, cpiConfigMapForMinorVersion)
	if err != nil {
		log.Error(err, "failed to fetch CPI template for cluster")
		return nil, fmt.Errorf(
			"failed to retrieve default AWS CPI manifests ConfigMap %q: %w",
			objName,
			err,
		)
	}

	cpiConfigMap := generateCPIConfigMapForCluster(cpiConfigMapForMinorVersion, cluster)
	if err := client.ServerSideApply(ctx, a.client, cpiConfigMap); err != nil {
		log.Error(err, "failed to apply CPI configmap for cluster")
		return nil, fmt.Errorf(
			"failed to apply AWS CPI manifests ConfigMap: %w",
			err,
		)
	}
	return cpiConfigMap, nil
}

func generateCPIConfigMapForCluster(
	cpiConfigMapForVersion *corev1.ConfigMap, cluster *clusterv1.Cluster,
) *corev1.ConfigMap {
	cpiConfigMapForCluster := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      fmt.Sprintf("%s-%s", cpiConfigMapForVersion.Name, cluster.Name),
		},
		Data: cpiConfigMapForVersion.Data,
	}
	return cpiConfigMapForCluster
}
