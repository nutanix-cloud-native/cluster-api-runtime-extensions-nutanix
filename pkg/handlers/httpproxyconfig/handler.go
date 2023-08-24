// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpproxyconfig

import (
	"context"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/topologymutation"

	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers"
)

const (
	// VariableName is http proxy external patch variable name.
	VariableName = "proxy"
)

type httpProxyConfigHandler struct {
	decoder   runtime.Decoder
	generator *systemdConfigGenerator
}

var (
	_ handlers.NamedHandler                     = &httpProxyConfigHandler{}
	_ handlers.DiscoverVariablesMutationHandler = &httpProxyConfigHandler{}
	_ handlers.GeneratePatchesMutationHandler   = &httpProxyConfigHandler{}
)

func New() *httpProxyConfigHandler {
	scheme := runtime.NewScheme()
	_ = bootstrapv1.AddToScheme(scheme)
	_ = controlplanev1.AddToScheme(scheme)
	return &httpProxyConfigHandler{
		decoder: serializer.NewCodecFactory(scheme).UniversalDecoder(
			controlplanev1.GroupVersion,
			bootstrapv1.GroupVersion,
		),
		generator: &systemdConfigGenerator{
			template: templates.Lookup("systemd.conf.tmpl"),
		},
	}
}

func (h *httpProxyConfigHandler) Name() string {
	return "http-proxy"
}

func (h *httpProxyConfigHandler) GeneratePatches(
	ctx context.Context,
	req *runtimehooksv1.GeneratePatchesRequest,
	resp *runtimehooksv1.GeneratePatchesResponse,
) {
	topologymutation.WalkTemplates(
		ctx,
		h.decoder,
		req,
		resp,
		func(ctx context.Context, obj runtime.Object, variables map[string]apiextensionsv1.JSON, holderRef runtimehooksv1.HolderReference) error {
			httpProxyVariable, found, err := GetVariable[HTTPProxyVariables](
				variables,
				VariableName,
			)
			if err != nil {
				return err
			}
			if !found {
				return fmt.Errorf("missing variable %q value", VariableName)
			}

			controlPlaneSelector := clusterv1.PatchSelector{
				APIVersion: controlplanev1.GroupVersion.String(),
				Kind:       "KubeadmControlPlaneTemplate",
				MatchResources: clusterv1.PatchSelectorMatch{
					ControlPlane: true,
				},
			}
			if err := generatePatch(obj, variables, holderRef, controlPlaneSelector, func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
				var err error
				obj.Spec.Template.Spec.KubeadmConfigSpec.Files, err = h.generator.AddSystemdFiles(
					httpProxyVariable, obj.Spec.Template.Spec.KubeadmConfigSpec.Files)
				return err
			}); err != nil {
				return err
			}

			defaultWorkerSelector := clusterv1.PatchSelector{
				APIVersion: bootstrapv1.GroupVersion.String(),
				Kind:       "KubeadmConfigTemplate",
				MatchResources: clusterv1.PatchSelectorMatch{
					MachineDeploymentClass: &clusterv1.PatchSelectorMatchMachineDeploymentClass{
						Names: []string{
							"default-worker",
						},
					},
				},
			}
			if err := generatePatch(obj, variables, holderRef, defaultWorkerSelector, func(obj *bootstrapv1.KubeadmConfigTemplate) error {
				var err error
				obj.Spec.Template.Spec.Files, err = h.generator.AddSystemdFiles(httpProxyVariable, obj.Spec.Template.Spec.Files)
				return err
			}); err != nil {
				return err
			}

			return nil
		},
	)
}

func (h *httpProxyConfigHandler) DiscoverVariables(
	ctx context.Context,
	_ *runtimehooksv1.DiscoverVariablesRequest,
	resp *runtimehooksv1.DiscoverVariablesResponse,
) {
	variable := HTTPProxyVariables{}
	resp.Variables = append(resp.Variables, clusterv1.ClusterClassVariable{
		Name:     VariableName,
		Required: false,
		Schema:   variable.VariableSchema(),
	})
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

func generatePatch[T runtime.Object](
	obj runtime.Object,
	variables map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	patchSelector clusterv1.PatchSelector,
	mutFn func(T) error,
) error {
	typed, ok := obj.(T)
	if !ok {
		return nil
	}

	if !matchSelector(patchSelector, obj, holderRef, variables) {
		return nil
	}

	return mutFn(typed)
}
