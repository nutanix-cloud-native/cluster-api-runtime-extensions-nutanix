// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpproxy

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/topologymutation"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/d2iq-labs/capi-runtime-extensions/pkg/capi"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers"
)

const (
	// HandlerNameInject is the name of the inject handler.
	HandlerNameInject = "HTTPProxyPatch"
)

type httpProxyPatchHandler struct {
	decoder   runtime.Decoder
	generator *systemdConfigGenerator
}

var (
	_ handlers.NamedHandler                   = &httpProxyPatchHandler{}
	_ handlers.GeneratePatchesMutationHandler = &httpProxyPatchHandler{}
)

func NewPatch() *httpProxyPatchHandler {
	scheme := runtime.NewScheme()
	_ = bootstrapv1.AddToScheme(scheme)
	_ = controlplanev1.AddToScheme(scheme)
	return &httpProxyPatchHandler{
		decoder: serializer.NewCodecFactory(scheme).UniversalDecoder(
			controlplanev1.GroupVersion,
			bootstrapv1.GroupVersion,
		),
		generator: &systemdConfigGenerator{
			template: templates.Lookup("systemd.conf.tmpl"),
		},
	}
}

func (h *httpProxyPatchHandler) Name() string {
	return HandlerNameInject
}

func (h *httpProxyPatchHandler) GeneratePatches(
	ctx context.Context,
	req *runtimehooksv1.GeneratePatchesRequest,
	resp *runtimehooksv1.GeneratePatchesResponse,
) {
	topologymutation.WalkTemplates(
		ctx,
		h.decoder,
		req,
		resp,
		func(
			ctx context.Context,
			obj runtime.Object,
			variables map[string]apiextensionsv1.JSON,
			holderRef runtimehooksv1.HolderReference,
		) error {
			log := ctrl.LoggerFrom(ctx).WithValues(
				"holderRef", holderRef,
			)

			httpProxyVariable, found, err := capi.GetVariable[HTTPProxyVariables](
				variables,
				VariableName,
			)
			if err != nil {
				return err
			}
			if !found {
				log.Info("http proxy variable not defined")
				return nil
			}

			log = log.WithValues("httpProxyVariable", httpProxyVariable)

			controlPlaneSelector := clusterv1.PatchSelector{
				APIVersion: controlplanev1.GroupVersion.String(),
				Kind:       "KubeadmControlPlaneTemplate",
				MatchResources: clusterv1.PatchSelectorMatch{
					ControlPlane: true,
				},
			}
			if err := generatePatch(
				obj, variables, holderRef, controlPlaneSelector, log,
				func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
					var err error
					log.WithValues("namespacedName", types.NamespacedName{
						Name:      obj.Name,
						Namespace: obj.Namespace,
					}).Info("adding files to kubeadm config spec")
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
			if err := generatePatch(
				obj, variables, holderRef, defaultWorkerSelector, log,
				func(obj *bootstrapv1.KubeadmConfigTemplate) error {
					var err error
					log.WithValues("namespacedName", types.NamespacedName{
						Name:      obj.Name,
						Namespace: obj.Namespace,
					}).Info("adding files to worker node kubeadm config template")
					obj.Spec.Template.Spec.Files, err = h.generator.AddSystemdFiles(httpProxyVariable, obj.Spec.Template.Spec.Files)
					return err
				}); err != nil {
				return err
			}

			return nil
		},
	)
}

func generatePatch[T runtime.Object](
	obj runtime.Object,
	variables map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	patchSelector clusterv1.PatchSelector,
	log logr.Logger,
	mutFn func(T) error,
) error {
	typed, ok := obj.(T)
	if !ok {
		log.V(5).WithValues(
			"objType", fmt.Sprintf("%T", obj),
			"expectedType", fmt.Sprintf("%T", *new(T)),
		).Info("not matching type")
		return nil
	}

	if !matchSelector(patchSelector, obj, holderRef, variables) {
		log.WithValues("selector", patchSelector).Info("not matching selector")
		return nil
	}

	return mutFn(typed)
}
