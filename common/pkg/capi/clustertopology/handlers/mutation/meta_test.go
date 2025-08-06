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
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ client.ObjectKey,
	_ ClusterGetter,
) error {
	if h.returnErr {
		return fmt.Errorf("This is a failure")
	}

	varAVal, ok := vars["varA"]
	if !ok {
		return fmt.Errorf("varA not found in vars")
	}
	if string(varAVal.Raw) != `{"a":1,"b":2}` {
		return fmt.Errorf("varA value mismatch: %s", string(varAVal.Raw))
	}

	varBVal, ok := vars["varB"]
	if !ok {
		return fmt.Errorf("varB not found in vars")
	}
	switch obj.GetKind() {
	case "KubeadmControlPlaneTemplate":
		if string(varBVal.Raw) != `{"c":3,"d":{"e":4,"f":5}}` {
			return fmt.Errorf("varB value mismatch: %s", string(varBVal.Raw))
		}
	case "KubeadmConfigTemplate":
		if string(varBVal.Raw) != `{"c":3,"d":{"e":5,"f":5}}` {
			return fmt.Errorf("varB value mismatch: %s", string(varBVal.Raw))
		}
	default:
		return fmt.Errorf("unexpected object kind: %s", obj.GetKind())
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
		vars,
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

func globalVars() []runtimehooksv1.Variable {
	return []runtimehooksv1.Variable{{
		Name: "varA",
		Value: apiextensionsv1.JSON{
			Raw: []byte(`{"a": 1, "b": 2}`),
		},
	}, {
		Name: "varB",
		Value: apiextensionsv1.JSON{
			Raw: []byte(`{"c": 3, "d": {"e": 4, "f": 5}}`),
		},
	}}
}

func overrideVars() []runtimehooksv1.Variable {
	return []runtimehooksv1.Variable{{
		Name: "builtin",
		Value: apiextensionsv1.JSON{
			Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
		},
	}, {
		Name: "varB",
		Value: apiextensionsv1.JSON{
			Raw: []byte(`{"d": {"e": 5}}`),
		},
	}}
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := gomega.NewWithT(t)

			h := NewMetaGeneratePatchesHandler("", nil, tt.mutaters...).(GeneratePatches)

			resp := &runtimehooksv1.GeneratePatchesResponse{}
			kctReq := request.NewKubeadmConfigTemplateRequestItem("kubeadm-config")
			kctReq.Variables = overrideVars()
			h.GeneratePatches(context.Background(), &runtimehooksv1.GeneratePatchesRequest{
				Variables: globalVars(),
				Items: []runtimehooksv1.GeneratePatchesRequestItem{
					kctReq,
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

func TestMergeVariableDefinitions(t *testing.T) {
	t.Parallel()

	type args struct {
		vars       map[string]apiextensionsv1.JSON
		globalVars map[string]apiextensionsv1.JSON
	}
	tests := []struct {
		name      string
		args      args
		want      map[string]apiextensionsv1.JSON
		wantErr   bool
		errString string
	}{
		{
			name: "no overlap, globalVars added",
			args: args{
				vars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`1`)},
				},
				globalVars: map[string]apiextensionsv1.JSON{
					"b": {Raw: []byte(`2`)},
				},
			},
			want: map[string]apiextensionsv1.JSON{
				"a": {Raw: []byte(`1`)},
				"b": {Raw: []byte(`2`)},
			},
		},
		{
			name: "globalVars value is nil, skipped",
			args: args{
				vars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`1`)},
				},
				globalVars: map[string]apiextensionsv1.JSON{
					"b": {Raw: nil},
				},
			},
			want: map[string]apiextensionsv1.JSON{
				"a": {Raw: []byte(`1`)},
			},
		},
		{
			name: "existing value is nil, globalVars value used",
			args: args{
				vars: map[string]apiextensionsv1.JSON{
					"a": {Raw: nil},
				},
				globalVars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`2`)},
				},
			},
			want: map[string]apiextensionsv1.JSON{
				"a": {Raw: []byte(`2`)},
			},
		},
		{
			name: "both values are scalars, globalVars ignored",
			args: args{
				vars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`1`)},
				},
				globalVars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`2`)},
				},
			},
			want: map[string]apiextensionsv1.JSON{
				"a": {Raw: []byte(`1`)},
			},
		},
		{
			name: "both values are objects, merged",
			args: args{
				vars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{"x":1,"y":2}`)},
				},
				globalVars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{"y":3,"z":4}`)},
				},
			},
			want: map[string]apiextensionsv1.JSON{
				"a": {Raw: []byte(`{"x":1,"y":2,"z":4}`)},
			},
		},
		{
			name: "both values are objects with nested objects, merged",
			args: args{
				vars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{"x":1,"y":{"a": 2,"b":{"c": 3}}}`)},
				},
				globalVars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{"y":{"a": 2,"b":{"c": 5, "d": 6}},"z":4}`)},
				},
			},
			want: map[string]apiextensionsv1.JSON{
				"a": {Raw: []byte(`{"x":1,"y":{"a": 2,"b":{"c": 3, "d": 6}},"z":4}`)},
			},
		},
		{
			name: "both values are objects with nested objects with vars having nil object explicitly, merged",
			args: args{
				vars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{"x":1,"y":{"a": 2,"b": null}}`)},
				},
				globalVars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{"y":{"a": 2,"b":{"c": 5, "d": 6}},"z":4}`)},
				},
			},
			want: map[string]apiextensionsv1.JSON{
				"a": {Raw: []byte(`{"x":1,"y":{"a": 2,"b":{"c": 5, "d": 6}},"z":4}`)},
			},
		},
		{
			name: "globalVars is scalar, vars is object, keep object",
			args: args{
				vars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{"x":1}`)},
				},
				globalVars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`2`)},
				},
			},
			want: map[string]apiextensionsv1.JSON{
				"a": {Raw: []byte(`{"x":1}`)},
			},
		},
		{
			name: "vars is scalar, globalVars is object, keep scalar",
			args: args{
				vars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`2`)},
				},
				globalVars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{"x":1}`)},
				},
			},
			want: map[string]apiextensionsv1.JSON{
				"a": {Raw: []byte(`2`)},
			},
		},
		{
			name: "invalid JSON in vars",
			args: args{
				vars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{invalid}`)},
				},
				globalVars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{"x":1}`)},
				},
			},
			wantErr:   true,
			errString: "failed to unmarshal existing value for key \"a\"",
		},
		{
			name: "invalid JSON in globalVars",
			args: args{
				vars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{"x":1}`)},
				},
				globalVars: map[string]apiextensionsv1.JSON{
					"a": {Raw: []byte(`{invalid}`)},
				},
			},
			wantErr:   true,
			errString: "failed to unmarshal global value for key \"a\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			got, err := mergeVariableOverridesWithGlobal(tt.args.vars, tt.args.globalVars)
			if tt.wantErr {
				g.Expect(err).To(gomega.HaveOccurred())
				g.Expect(err.Error()).To(gomega.ContainSubstring(tt.errString))
				return
			}
			g.Expect(err).ToNot(gomega.HaveOccurred())
			// Compare JSON values
			for k, wantVal := range tt.want {
				gotVal, ok := got[k]
				g.Expect(ok).To(gomega.BeTrue(), "missing key %q", k)
				var wantObj, gotObj interface{}
				_ = json.Unmarshal(wantVal.Raw, &wantObj)
				_ = json.Unmarshal(gotVal.Raw, &gotObj)
				g.Expect(gotObj).To(gomega.Equal(wantObj), "key %q", k)
			}
			// Check for unexpected keys
			g.Expect(len(got)).To(gomega.Equal(len(tt.want)))
		})
	}
}
