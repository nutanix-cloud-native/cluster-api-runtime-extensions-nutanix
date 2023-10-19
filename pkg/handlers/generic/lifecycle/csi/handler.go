// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package csi

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	utilyaml "sigs.k8s.io/cluster-api/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	commonhandlers "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
	lifecycleutils "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/lifecycle/utils"
)

const (
	variableRootName = "csi"
	kindStorageClass = "StorageClass"
)

var (
	defualtStorageClassKey = "storageclass.kubernetes.io/is-default-class"
	defaultStorageClassMap = map[string]string{
		defualtStorageClassKey: "true",
	}
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
		log.Info(fmt.Sprintf("Creating config map for csi provider %s", provider))
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
		if cm != nil {
			if provider.Name == csiProviders.DefaultClassName {
				log.Info("Setting default storage class ", provider, csiProviders.DefaultClassName)
				err = setDefaultStorageClass(log, cm)
				if err != nil {
					log.Error(err, "failed to set default storage class")
					resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
				}
				if err := c.client.Update(ctx, cm); err != nil {
					log.Error(err, "failed to apply default storage class annotation to configmap")
					resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
				}
			}
			err = lifecycleutils.EnsureCRSForClusterFromConfigMap(
				ctx,
				c.client,
				&req.Cluster,
				cm,
			)
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
}

func setDefaultStorageClass(
	log logr.Logger,
	cm *corev1.ConfigMap,
) error {
	for k, contents := range cm.Data {
		objs, err := utilyaml.ToUnstructured([]byte(contents))
		if err != nil {
			log.Error(err, "failed to parse yaml")
			continue
		}
		for i := range objs {
			obj := objs[i]
			if obj.GetKind() == kindStorageClass {
				obj.SetAnnotations(defaultStorageClassMap)
			}
		}
		rawObjs, err := utilyaml.FromUnstructured(objs)
		if err != nil {
			return fmt.Errorf("failed to convert unstructured objects back to string %w", err)
		}
		cm.Data[k] = string(rawObjs)
	}
	return nil
}
