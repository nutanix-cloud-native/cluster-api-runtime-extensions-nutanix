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
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/k8s/client"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/lifecycle/utils"
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
		prefix+".defaults-namespace",
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

func New(
	c ctrlclient.Client,
	cfg *NFDConfig,
) *DefaultNFD {
	return &DefaultNFD{
		client:       c,
		config:       cfg,
		variableName: clusterconfig.MetaVariableName,
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

	cm, err := n.ensureNFDConfigMapForCluster(ctx, &req.Cluster)
	if err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		log.Error(err, "failed to apply NFD ConfigMap for cluster")
		return
	}
	err = utils.EnsureCRSForClusterFromConfigMaps(ctx, cm.Name, n.client, &req.Cluster, cm)
	if err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		log.Error(err, "failed to apply NFD ClusterResourceSet for cluster")
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

// ensureNFDConfigMapForCluster is a private function that creates a configMap for the cluster.
func (n *DefaultNFD) ensureNFDConfigMapForCluster(
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
