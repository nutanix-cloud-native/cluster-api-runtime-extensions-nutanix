// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package csi

import (
	"context"
	"fmt"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	commonhandlers "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/k8s/client"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	crsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	variableRootName = "csiProviders"
)

type CSIProvider interface {
	EnsureCSIConfigMapForCluster(context.Context, *clusterv1.Cluster) (*corev1.ConfigMap, error)
}

type CSIHandler struct {
	client          ctrlclient.Client
	variableName    string
	variablePath    []string
	ProviderHandler map[string]CSIProvider
}

var (
	_ commonhandlers.Named                   = &CSIHandler{}
	_ lifecycle.AfterControlPlaneInitialized = &CSIHandler{}
)

func New(
	c ctrlclient.Client,
	handlers map[string]CSIProvider,
) *CSIHandler {
	return &CSIHandler{
		client:          c,
		variableName:    clusterconfig.MetaVariableName,
		variablePath:    []string{"addons", variableRootName},
		ProviderHandler: handlers,
	}
}

func (c *CSIHandler) Name() string {
	return "CSIHandler"
}

func (c *CSIHandler) AfterControlPlaneInitialized(
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
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
	csiProviders, found, err := variables.Get[v1alpha1.CSIProviders](
		varMap,
		c.variableName,
		c.variablePath...)
	if err != nil {
		log.Error(
			err,
			"failed to read CSI provider from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read CSI provider from cluster definition: %v",
				err,
			),
		)
		return
	}
	if !found || csiProviders.Providers == nil || len(csiProviders.Providers) == 0 {
		log.V(4).Info(
			fmt.Sprintf(
				"Skipping CSI handler, no providers given in %v",
				csiProviders,
			),
		)
		return
	}
	for _, provider := range csiProviders.Providers {
		handler, ok := c.ProviderHandler[provider.Name]
		if !ok {
			log.V(4).Info(
				fmt.Sprintf(
					"Skipping CSI handler, for provider given in %q. Provider handler not given ",
					provider,
				),
			)
			continue
		}
		cm, err := handler.EnsureCSIConfigMapForCluster(ctx, &req.Cluster)
		if err != nil {
			log.Error(
				err,
				fmt.Sprintf(
					"failed to ensure %s csi driver installation manifests ConfigMap",
					provider.Name,
				),
			)
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		}
		err = c.EnsureCSICRSForCluster(ctx, &req.Cluster, cm)
		if err != nil {
			log.Error(
				err,
				fmt.Sprintf(
					"failed to ensure %s csi driver installation manifests ConfigMap",
					provider.Name,
				),
			)
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		}
	}
}

func (c *CSIHandler) EnsureCSICRSForCluster(
	ctx context.Context,
	cluster *clusterv1.Cluster,
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
				MatchLabels: map[string]string{clusterv1.ClusterNameLabel: cluster.Name},
			},
		},
	}

	if err := controllerutil.SetOwnerReference(cluster, crs, c.client.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}
	err := client.ServerSideApply(ctx, c.client, crs)
	if err != nil {
		return fmt.Errorf("failed to server side apply %w", err)
	}
	return nil
}
