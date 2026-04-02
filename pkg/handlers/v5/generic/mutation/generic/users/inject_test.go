// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package users

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/utils/ptr"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/eks/mutation/testutils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func Test_generateBootstrapUser(t *testing.T) {
	type args struct {
		userFromVariable v1alpha1.User
	}
	tests := []struct {
		name string
		args args
		want bootstrapv1.User
	}{
		{
			name: "if user sets hashed password, enable password auth and set passwd",
			args: args{
				userFromVariable: v1alpha1.User{
					Name:           "example",
					HashedPassword: "example",
				},
			},
			want: bootstrapv1.User{
				Name:         "example",
				Passwd:       "example",
				LockPassword: ptr.To(false),
			},
		},
		{
			name: "if user does not set hashed password, disable password auth and do not set passwd",
			args: args{
				userFromVariable: v1alpha1.User{
					Name: "example",
				},
			},
			want: bootstrapv1.User{
				Name:         "example",
				LockPassword: ptr.To(true),
			},
		},
		{
			name: "if user sets empty hashed password, disable password auth and do not set passwd",
			args: args{
				userFromVariable: v1alpha1.User{
					Name:           "example",
					HashedPassword: "",
				},
			},
			want: bootstrapv1.User{
				Name:         "example",
				LockPassword: ptr.To(true),
			},
		},
		{
			name: "if user sets sudo, include it in the patch",
			args: args{
				userFromVariable: v1alpha1.User{
					Name: "example",
					Sudo: "example",
				},
			},
			want: bootstrapv1.User{
				Name:         "example",
				Sudo:         "example",
				LockPassword: ptr.To(true),
			},
		},
		{
			name: "if user does not set sudo, do not include in the patch",
			args: args{
				userFromVariable: v1alpha1.User{
					Name: "example",
				},
			},
			want: bootstrapv1.User{
				Name:         "example",
				LockPassword: ptr.To(true),
			},
		},
		{
			name: "if user sets empty sudo, do not include in the patch",
			args: args{
				userFromVariable: v1alpha1.User{
					Name: "example",
					Sudo: "",
				},
			},
			want: bootstrapv1.User{
				Name:         "example",
				LockPassword: ptr.To(true),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateBootstrapUser(tt.args.userFromVariable)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("generateBootstrapUser() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

var (
	testUser1 = v1alpha1.User{
		Name:           "complete",
		HashedPassword: "password",
		SSHAuthorizedKeys: []string{
			"key1",
			"key2",
		},
		Sudo: "ALL=(ALL) NOPASSWD:ALL",
	}
	testUser2 = v1alpha1.User{
		Name: "onlyname",
	}
)

func TestUsersPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "Users mutator suite")
}

var _ = Describe("Generate Users patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", helpers.TestEnv.Client, NewPatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name: "users set for KubeadmControlPlaneTemplate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					[]v1alpha1.User{testUser1, testUser2},
					VariableName,
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/kubeadmConfigSpec/users",
				ValueMatcher: gomega.HaveLen(2),
			}},
		},
		{
			Name: "users set for KubeadmConfigTemplate generic worker",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					[]v1alpha1.User{testUser1, testUser2},
					VariableName,
				),
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					map[string]any{
						"machineDeployment": map[string]any{
							"class": names.SimpleNameGenerator.GenerateName("worker-"),
						},
					},
				),
			},
			RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/users",
				ValueMatcher: gomega.HaveLen(2),
			}},
		},
		{
			Name: "users set for NodeadmConfigTemplate generic worker",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					[]v1alpha1.User{testUser1, testUser2},
					VariableName,
				),
				capitest.VariableWithValue(
					"builtin",
					map[string]any{
						"machineDeployment": map[string]any{
							"class": names.SimpleNameGenerator.GenerateName("worker-"),
						},
					},
				),
			},
			RequestItem: testutils.NewNodeadmConfigTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/users",
				ValueMatcher: gomega.HaveLen(2),
			}},
		},
	}

	// create test node for each case
	for _, tt := range testDefs {
		It(tt.Name, func() {
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &tt)
		})
	}
})
