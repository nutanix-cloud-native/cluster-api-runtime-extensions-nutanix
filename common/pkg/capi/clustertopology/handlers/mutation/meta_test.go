// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/onsi/gomega"
	"gomodules.xyz/jsonpatch/v2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
)

type testHandler struct {
	returnErr          bool
	mutateControlPlane bool
}

var _ MetaMutator = &testHandler{}

func (h *testHandler) Mutate(
	_ context.Context,
	obj *unstructured.Unstructured,
	_ map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ client.ObjectKey,
	_ ClusterGetter,
) error {
	if h.returnErr {
		return fmt.Errorf("This is a failure")
	}

	if h.mutateControlPlane {
		return patches.MutateIfApplicable(
			obj, nil, &holderRef, selectors.ControlPlane(), logr.Discard(),
			func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
				obj.Spec.Template.Spec.KubeadmConfigSpec.PostKubeadmCommands = append(
					obj.Spec.Template.Spec.KubeadmConfigSpec.PostKubeadmCommands,
					fmt.Sprintf(
						"control-plane-extra-post-kubeadm-%d",
						len(obj.Spec.Template.Spec.KubeadmConfigSpec.PostKubeadmCommands),
					),
				)
				return nil
			},
		)
	}

	return patches.MutateIfApplicable(
		obj,
		machineVars(),
		&holderRef,
		selectors.WorkersKubeadmConfigTemplateSelector(),
		logr.Discard(),
		func(obj *bootstrapv1.KubeadmConfigTemplate) error {
			obj.Spec.Template.Spec.PostKubeadmCommands = append(
				obj.Spec.Template.Spec.PostKubeadmCommands,
				fmt.Sprintf(
					"worker-extra-post-kubeadm-%d",
					len(obj.Spec.Template.Spec.PostKubeadmCommands),
				),
			)
			return nil
		},
	)
}

func machineVars() map[string]apiextensionsv1.JSON {
	return map[string]apiextensionsv1.JSON{
		"builtin": {Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`)},
	}
}

func TestMetaGeneratePatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		mutaters         []MetaMutator
		expectedResponse *runtimehooksv1.GeneratePatchesResponse
	}{{
		name: "no handlers",
		expectedResponse: &runtimehooksv1.GeneratePatchesResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusSuccess,
			},
			Items: []runtimehooksv1.GeneratePatchesResponseItem{{
				UID:       "kubeadm-config",
				PatchType: runtimehooksv1.JSONPatchType,
				Patch:     jsonPatch([]jsonpatch.Operation{}...),
			}, {
				UID:       "kubeadm-control-plane",
				PatchType: runtimehooksv1.JSONPatchType,
				Patch:     jsonPatch([]jsonpatch.Operation{}...),
			}},
		},
	}, {
		name: "single success handler",
		mutaters: []MetaMutator{
			&testHandler{},
		},
		expectedResponse: &runtimehooksv1.GeneratePatchesResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusSuccess,
			},
			Items: []runtimehooksv1.GeneratePatchesResponseItem{{
				UID:       "kubeadm-config",
				PatchType: runtimehooksv1.JSONPatchType,
				Patch: jsonPatch(
					jsonpatch.NewOperation(
						"add",
						"/spec/template/spec/postKubeadmCommands/1",
						"worker-extra-post-kubeadm-1",
					),
				),
			}, {
				UID:       "kubeadm-control-plane",
				PatchType: runtimehooksv1.JSONPatchType,
				Patch:     jsonPatch([]jsonpatch.Operation{}...),
			}},
		},
	}, {
		name: "single failure handler",
		mutaters: []MetaMutator{
			&testHandler{
				returnErr: true,
			},
		},
		expectedResponse: &runtimehooksv1.GeneratePatchesResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusFailure,
				Message: "This is a failure",
			},
		},
	}, {
		name: "multiple success handlers",
		mutaters: []MetaMutator{
			&testHandler{},
			&testHandler{},
			&testHandler{mutateControlPlane: true},
		},
		expectedResponse: &runtimehooksv1.GeneratePatchesResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusSuccess,
			},
			Items: []runtimehooksv1.GeneratePatchesResponseItem{{
				UID:       "kubeadm-config",
				PatchType: runtimehooksv1.JSONPatchType,
				Patch: jsonPatch(
					jsonpatch.NewOperation(
						"add",
						"/spec/template/spec/postKubeadmCommands/1",
						"worker-extra-post-kubeadm-1",
					),
					jsonpatch.NewOperation(
						"add",
						"/spec/template/spec/postKubeadmCommands/2",
						"worker-extra-post-kubeadm-2",
					),
				),
			}, {
				UID:       "kubeadm-control-plane",
				PatchType: runtimehooksv1.JSONPatchType,
				Patch: jsonPatch(
					jsonpatch.NewOperation(
						"add",
						"/spec/template/spec/kubeadmConfigSpec/postKubeadmCommands",
						[]string{"control-plane-extra-post-kubeadm-0"},
					)),
			}},
		},
	}, {
		name: "success handler followed by failure handler",
		mutaters: []MetaMutator{
			&testHandler{},
			&testHandler{
				returnErr: true,
			},
		},
		expectedResponse: &runtimehooksv1.GeneratePatchesResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusFailure,
				Message: "This is a failure",
			},
		},
	}, {
		name: "failure handler followed by success handler",
		mutaters: []MetaMutator{
			&testHandler{
				returnErr: true,
			},
			&testHandler{},
		},
		expectedResponse: &runtimehooksv1.GeneratePatchesResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusFailure,
				Message: `This is a failure`,
			},
		},
	}}

	for idx := range tests {
		tt := tests[idx]

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := gomega.NewWithT(t)

			h := NewMetaGeneratePatchesHandler("", nil, tt.mutaters...).(GeneratePatches)

			resp := &runtimehooksv1.GeneratePatchesResponse{}
			h.GeneratePatches(context.Background(), &runtimehooksv1.GeneratePatchesRequest{
				Items: []runtimehooksv1.GeneratePatchesRequestItem{
					request.NewKubeadmConfigTemplateRequestItem("kubeadm-config"),
					request.NewKubeadmControlPlaneTemplateRequestItem("kubeadm-control-plane"),
				},
			}, resp)

			g.Expect(resp).To(gomega.Equal(tt.expectedResponse))
		})
	}
}

func jsonPatch(operations ...jsonpatch.Operation) []byte {
	b, _ := json.Marshal( //nolint:errchkjson // OK to do this in a test when the type can be marshalled.
		operations,
	)
	return b
}
