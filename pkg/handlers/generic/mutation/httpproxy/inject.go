// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpproxy

import (
	"context"
	"fmt"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/topologymutation"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/apis"
	commonhandlers "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
)

const (
	// HandlerNamePatch is the name of the inject handler.
	HandlerNamePatch = "HTTPProxyPatch"

	// instanceMetadataIP is the IPv4 address used to retrieve
	// instance metadata in AWS, Azure, OpenStack, etc.
	instanceMetadataIP = "169.254.169.254"
)

type httpProxyPatchHandler struct {
	client ctrlclient.Reader

	variableName      string
	variableFieldPath []string
}

var (
	_ commonhandlers.Named     = &httpProxyPatchHandler{}
	_ mutation.GeneratePatches = &httpProxyPatchHandler{}
)

func NewPatch(
	cl ctrlclient.Reader,
) *httpProxyPatchHandler {
	return newHTTPProxyPatchHandler(cl, VariableName)
}

func NewMetaPatch(
	cl ctrlclient.Reader,
) *httpProxyPatchHandler {
	return newHTTPProxyPatchHandler(cl, clusterconfig.MetaVariableName, VariableName)
}

func newHTTPProxyPatchHandler(
	cl ctrlclient.Reader,
	variableName string,
	variableFieldPath ...string,
) *httpProxyPatchHandler {
	return &httpProxyPatchHandler{
		client:            cl,
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *httpProxyPatchHandler) Name() string {
	return HandlerNamePatch
}

func (h *httpProxyPatchHandler) Mutate(
	ctx context.Context,
	obj runtime.Object,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	clusterKey ctrlclient.ObjectKey,
) error {
	log := ctrl.LoggerFrom(ctx, "holderRef", holderRef)

	noProxy, err := h.detectNoProxy(ctx, clusterKey)
	if err != nil {
		log.Error(err, "failed to resolve no proxy value")
	}

	httpProxyVariable, found, err := variables.Get[v1alpha1.HTTPProxy](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		return err
	}
	if !found {
		log.Info("http proxy variable not defined")
		return nil
	}

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		httpProxyVariable,
	)

	if err := patches.Generate(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", ctrlclient.ObjectKeyFromObject(obj),
			).Info("adding files to control plane kubeadm config spec")
			obj.Spec.Template.Spec.KubeadmConfigSpec.Files = append(
				obj.Spec.Template.Spec.KubeadmConfigSpec.Files,
				generateSystemdFiles(httpProxyVariable, noProxy)...,
			)
			return nil
		}); err != nil {
		return err
	}

	if err := patches.Generate(
		obj, vars, &holderRef, selectors.WorkersKubeadmConfigTemplateSelector(), log,
		func(obj *bootstrapv1.KubeadmConfigTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", ctrlclient.ObjectKeyFromObject(obj),
			).Info("adding files to worker node kubeadm config template")
			obj.Spec.Template.Spec.Files = append(
				obj.Spec.Template.Spec.Files,
				generateSystemdFiles(httpProxyVariable, noProxy)...,
			)
			return nil
		}); err != nil {
		return err
	}

	return nil
}

func (h *httpProxyPatchHandler) GeneratePatches(
	ctx context.Context,
	req *runtimehooksv1.GeneratePatchesRequest,
	resp *runtimehooksv1.GeneratePatchesResponse,
) {
	clusterKey := commonhandlers.ClusterKeyFromReq(req)

	topologymutation.WalkTemplates(
		ctx,
		apis.CAPIDecoder(),
		req,
		resp,
		func(
			ctx context.Context,
			obj runtime.Object,
			vars map[string]apiextensionsv1.JSON,
			holderRef runtimehooksv1.HolderReference,
		) error {
			return h.Mutate(ctx, obj, vars, holderRef, clusterKey)
		},
	)
}

func (h *httpProxyPatchHandler) detectNoProxy(
	ctx context.Context,
	clusterKey ctrlclient.ObjectKey,
) ([]string, error) {
	cluster := &capiv1.Cluster{}
	if err := h.client.Get(ctx, clusterKey, cluster); err != nil {
		return nil, err
	}

	return generateNoProxy(cluster), nil
}

// generateNoProxy creates default NO_PROXY values that should be applied on cluster
// in any environment and are preventing the use of proxy for cluster internal
// networking.
func generateNoProxy(cluster *capiv1.Cluster) []string {
	noProxy := []string{
		"localhost",
		"127.0.0.1",
	}

	if cluster.Spec.ClusterNetwork != nil &&
		cluster.Spec.ClusterNetwork.Pods != nil {
		noProxy = append(noProxy, cluster.Spec.ClusterNetwork.Pods.CIDRBlocks...)
	}

	if cluster.Spec.ClusterNetwork != nil &&
		cluster.Spec.ClusterNetwork.Services != nil {
		noProxy = append(noProxy, cluster.Spec.ClusterNetwork.Services.CIDRBlocks...)
	}

	serviceDomain := "cluster.local"
	if cluster.Spec.ClusterNetwork != nil &&
		cluster.Spec.ClusterNetwork.ServiceDomain != "" {
		serviceDomain = cluster.Spec.ClusterNetwork.ServiceDomain
	}

	noProxy = append(
		noProxy,
		"kubernetes",
		"kubernetes.default",
		".svc",
		// append .svc.<SERVICE_DOMAIN>
		fmt.Sprintf(".svc.%s", strings.TrimLeft(serviceDomain, ".")),
	)

	if cluster.Spec.InfrastructureRef == nil {
		return noProxy
	}

	// Add infra-specific entries
	switch cluster.Spec.InfrastructureRef.Kind {
	case "AWSCluster", "AWSManagedCluster":
		noProxy = append(
			noProxy,
			// Exclude the instance metadata service
			instanceMetadataIP,
			// Exclude the control plane endpoint
			".elb.amazonaws.com",
		)
	case "AzureCluster", "AzureManagedControlPlane":
		noProxy = append(
			noProxy,
			// Exclude the instance metadata service
			instanceMetadataIP,
		)
	case "GCPCluster":
		noProxy = append(
			noProxy,
			// Exclude the instance metadata service
			instanceMetadataIP,
			// Exclude aliases for instance metadata service.
			// See https://cloud.google.com/vpc/docs/special-configurations
			"metadata",
			"metadata.google.internal",
		)
	default:
		// Unknown infrastructure. Do nothing.
	}
	return noProxy
}
