// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nfd

import (
	"context"
	"fmt"

	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	crsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/k8s/client"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers"
)

type NFDConfig struct {
	defaultsNamespace   string
	defaultNFDConfigMap string
}

type DefaultNFD struct {
	client ctrlclient.Client
	config *NFDConfig

	variableName string   // points to the global config variable
	variablePath []string // path of this variable on the global config variable
}

func (n *NFDConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&n.defaultsNamespace,
		prefix+".defaultsNamespace",
		corev1.NamespaceDefault,
		"namespace location of ConfigMap used to deploy Node Feature Discovery (NFD).",
	)
	flags.StringVar(
		&n.defaultNFDConfigMap,
		prefix+".default-nfd-configmap-name",
		"node-feature-discovery",
		"name of the ConfigMap used to deploy Node Feature Discovery (NFD)",
	)
}

const (
	variableName = "nfd"
)

func NewMetaHandler(
	c ctrlclient.Client,
	cfg *NFDConfig,
) *DefaultNFD {
	return &DefaultNFD{
		client:       c,
		config:       cfg,
		variableName: handlers.MetaVariableName,
		variablePath: []string{"addons", variableName},
	}
}

func (n *DefaultNFD) Name() string {
	return "DefaultNFD"
}

func (n *DefaultNFD) AfterControlPlaneInitialized(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	resp *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	clusterKey := ctrlclient.ObjectKeyFromObject(&req.Cluster)

	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		clusterKey,
	)
	varMap := variables.ClusterVariablesToVariablesMap(req.Cluster.Spec.Topology.Variables)

	_, found, err := variables.Get[v1alpha1.NFD](varMap, n.variableName, n.variablePath...)
	if err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		log.Error(err, "failed to get NFD variable")
		return
	}
	// If the variable isn't there or disabled we can ignore it.
	if !found {
		log.V(4).Info(
			"Skipping NFD handler. Not specified in cluster config.",
		)
		return
	}

	cm, err := n.ensureNFDConfigmapForCluster(ctx, &req.Cluster)
	if err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		log.Error(err, "failed to apply NFD ConfigMap for cluster")
		return
	}
	err = n.ensureNFDCRSForCluster(ctx, &req.Cluster, cm)
	if err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		log.Error(err, "failed to apply NFD ClusterResourceSet for cluster")
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

// ensureNFDConfigmapForCluster is a private function that creates a configMap for the cluster.
func (n *DefaultNFD) ensureNFDConfigmapForCluster(
	ctx context.Context,
	cluster *capiv1.Cluster,
) (*corev1.ConfigMap, error) {
	key := ctrlclient.ObjectKey{
		Namespace: n.config.defaultsNamespace,
		Name:      n.config.defaultNFDConfigMap,
	}
	cm := &corev1.ConfigMap{}
	err := n.client.Get(ctx, key, cm)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to fetch the configmap specified by %v: %w",
			n.config,
			err,
		)
	}
	// Base configmap is there now we create one in the cluster namespace if needed.
	cmForCluster := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      n.config.defaultNFDConfigMap,
		},
		Data: cm.Data,
	}
	err = client.ServerSideApply(ctx, n.client, cmForCluster)
	if err != nil {
		return nil, fmt.Errorf("failed to apply NFD ConfigMap for cluster: %w", err)
	}
	return cmForCluster, nil
}

func (n *DefaultNFD) ensureNFDCRSForCluster(
	ctx context.Context,
	cluster *capiv1.Cluster,
	cm *corev1.ConfigMap,
) error {
	crs := &crsv1.ClusterResourceSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: crsv1.GroupVersion.String(),
			Kind:       "ClusterResourceSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      cm.Name + "-" + cluster.Name,
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

	if err := controllerutil.SetOwnerReference(cluster, crs, n.client.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	err := client.ServerSideApply(ctx, n.client, crs)
	if err != nil {
		return fmt.Errorf("failed to server side apply %w", err)
	}
	return nil
}
