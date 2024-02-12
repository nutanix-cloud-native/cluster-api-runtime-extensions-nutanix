// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cilium

import (
	"context"
	"fmt"
	"maps"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	crsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/k8s/client"
)

type crsConfig struct {
	defaultsNamespace string

	defaultCiliumConfigMapName string
}

func (c *crsConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultsNamespace,
		prefix+".defaults-namespace",
		corev1.NamespaceDefault,
		"namespace of the ConfigMap used to deploy Cilium",
	)

	flags.StringVar(
		&c.defaultCiliumConfigMapName,
		prefix+".default-cilium-configmap-name",
		"cilium",
		"name of the ConfigMap used to deploy Cilium",
	)
}

type crsStrategy struct {
	config crsConfig

	client ctrlclient.Client
}

func (s crsStrategy) apply(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	log logr.Logger,
) error {
	defaultCiliumConfigMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: s.config.defaultsNamespace,
			Name:      s.config.defaultCiliumConfigMapName,
		},
	}

	err := s.client.Get(
		ctx,
		ctrlclient.ObjectKeyFromObject(defaultCiliumConfigMap),
		defaultCiliumConfigMap,
	)
	if err != nil {
		return fmt.Errorf("failed to get default Cilium ConfigMap: %w", err)
	}

	log.Info("Ensuring Cilium installation CRS and ConfigMap exist for cluster")
	if err := s.ensureCNICRSForCluster(ctx, &req.Cluster, defaultCiliumConfigMap); err != nil {
		log.Error(
			err,
			"failed to ensure Cilium installation manifests ConfigMap and ClusterResourceSet exist in cluster namespace",
		)
		return fmt.Errorf(
			"failed to ensure Cilium installation manifests ConfigMap and ClusterResourceSet exist in cluster namespace: %w",
			err,
		)
	}

	return nil
}

func (s crsStrategy) ensureCNICRSForCluster(
	ctx context.Context,
	cluster *capiv1.Cluster,
	defaultCiliumConfigMap *corev1.ConfigMap,
) error {
	cniObjs, err := generateCNICRS(
		defaultCiliumConfigMap,
		cluster,
		s.client.Scheme(),
	)
	if err != nil {
		return fmt.Errorf(
			"failed to generate Cilium provider CNI CRS: %w",
			err,
		)
	}

	if err := client.ServerSideApply(ctx, s.client, cniObjs...); err != nil {
		return fmt.Errorf(
			"failed to apply Cilium CNI installation CRS: %w",
			err,
		)
	}

	return nil
}

func generateCNICRS(
	ciliumConfigMap *corev1.ConfigMap,
	cluster *capiv1.Cluster,
	scheme *runtime.Scheme,
) ([]ctrlclient.Object, error) {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      "cilium-cni-installation-" + cluster.Name,
		},
	}
	cm.Data = maps.Clone(ciliumConfigMap.Data)

	if err := controllerutil.SetOwnerReference(cluster, cm, scheme); err != nil {
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	crs := &crsv1.ClusterResourceSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: crsv1.GroupVersion.String(),
			Kind:       "ClusterResourceSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      cm.Name,
		},
		Spec: crsv1.ClusterResourceSetSpec{
			Resources: []crsv1.ResourceRef{{
				Kind: string(crsv1.ConfigMapClusterResourceSetResourceKind),
				Name: cm.Name,
			}},
			Strategy: string(crsv1.ClusterResourceSetStrategyReconcile),
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{capiv1.ClusterNameLabel: cluster.Name},
			},
		},
	}

	if err := controllerutil.SetOwnerReference(cluster, crs, scheme); err != nil {
		return nil, fmt.Errorf("failed to set owner reference: %w", err)
	}

	return []ctrlclient.Object{cm, crs}, nil
}
