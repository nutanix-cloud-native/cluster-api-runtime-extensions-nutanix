// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package calico

import (
	"context"
	_ "embed"

	"github.com/go-logr/logr"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/topologymutation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/apis"
	commonhandlers "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	capav1 "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/cni"
)

const (
	// HandlerNamePatch is the name of the inject handler.
	HandlerNamePatch = "CalicoCNIPatch"
)

type calicoPatchHandler struct {
	variableName      string
	variableFieldPath []string
}

var (
	_ commonhandlers.Named     = &calicoPatchHandler{}
	_ mutation.GeneratePatches = &calicoPatchHandler{}
	_ mutation.MetaMutator     = &calicoPatchHandler{}
)

func NewPatch() *calicoPatchHandler {
	return newCalicoPatchHandler(cni.VariableName)
}

func NewMetaPatch() *calicoPatchHandler {
	return newCalicoPatchHandler(
		clusterconfig.MetaVariableName,
		"addons",
		cni.VariableName,
	)
}

func newCalicoPatchHandler(
	variableName string,
	variableFieldPath ...string,
) *calicoPatchHandler {
	return &calicoPatchHandler{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *calicoPatchHandler) Name() string {
	return HandlerNamePatch
}

func (h *calicoPatchHandler) Mutate(
	ctx context.Context,
	obj runtime.Object,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ client.ObjectKey,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	cniVar, found, err := variables.Get[v1alpha1.CNI](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		return err
	}
	if !found {
		log.V(5).Info("cni variable not defined")
		return nil
	}
	if cniVar.Provider != v1alpha1.CNIProviderCalico {
		log.V(5).
			WithValues("cniProvider", cniVar.Provider).
			Info("CNI provider not defined as Calico - skipping")
		return nil
	}

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		cniVar,
	)

	return patches.Generate(
		obj,
		vars,
		&holderRef,
		selectors.InfrastructureCluster(capav1.GroupVersion.Version, "AWSClusterTemplate"),
		log,
		mutateAWSClusterTemplateFunc(log),
	)
}

func (h *calicoPatchHandler) GeneratePatches(
	ctx context.Context,
	req *runtimehooksv1.GeneratePatchesRequest,
	resp *runtimehooksv1.GeneratePatchesResponse,
) {
	topologymutation.WalkTemplates(
		ctx,
		apis.DecoderForScheme(apis.CAPAScheme()),
		req,
		resp,
		func(
			ctx context.Context,
			obj runtime.Object,
			vars map[string]apiextensionsv1.JSON,
			holderRef runtimehooksv1.HolderReference,
		) error {
			return h.Mutate(ctx, obj, vars, holderRef, client.ObjectKey{})
		},
	)
}

func mutateAWSClusterTemplateFunc(log logr.Logger) func(obj *capav1.AWSClusterTemplate) error {
	return func(obj *capav1.AWSClusterTemplate) error {
		log.WithValues(
			"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
			"patchedObjectName", client.ObjectKeyFromObject(obj),
		).Info("setting CNI ingress rules in AWS cluster spec")

		if obj.Spec.Template.Spec.NetworkSpec.CNI == nil {
			obj.Spec.Template.Spec.NetworkSpec.CNI = &capav1.CNISpec{}
		}
		obj.Spec.Template.Spec.NetworkSpec.CNI.CNIIngressRules = append(
			obj.Spec.Template.Spec.NetworkSpec.CNI.CNIIngressRules,
			capav1.CNIIngressRule{
				Description: "typha (calico)",
				Protocol:    capav1.SecurityGroupProtocolTCP,
				FromPort:    5473,
				ToPort:      5473,
			},
			capav1.CNIIngressRule{
				Description: "bgp (calico)",
				Protocol:    capav1.SecurityGroupProtocolTCP,
				FromPort:    179,
				ToPort:      179,
			},
			capav1.CNIIngressRule{
				Description: "IP-in-IP (calico)",
				Protocol:    capav1.SecurityGroupProtocolIPinIP,
				FromPort:    -1,
				ToPort:      65535,
			},
			capav1.CNIIngressRule{
				Description: "node metrics (calico)",
				Protocol:    capav1.SecurityGroupProtocolTCP,
				FromPort:    9091,
				ToPort:      9091,
			},
			capav1.CNIIngressRule{
				Description: "typha metrics (calico)",
				Protocol:    capav1.SecurityGroupProtocolTCP,
				FromPort:    9093,
				ToPort:      9093,
			},
		)

		return nil
	}
}
