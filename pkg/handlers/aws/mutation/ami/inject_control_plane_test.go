// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ami

import (
	"testing"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	capav1 "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest/request"
	awsclusterconfig "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/aws/clusterconfig"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
	"github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
)

func TestControlPlaneAMIGeneratePatches(t *testing.T) {
	t.Parallel()

	testControlPlaneGeneratePatches(
		t,
		func() mutation.GeneratePatches { return NewControlPlanePatch() },
		clusterconfig.MetaVariableName,
		clusterconfig.MetaControlPlaneConfigName,
		awsclusterconfig.AWSVariableName,
		VariableName,
	)
}

func testControlPlaneGeneratePatches(
	t *testing.T,
	generatorFunc func() mutation.GeneratePatches,
	variableName string,
	variablePath ...string,
) {
	t.Helper()

	capitest.ValidateGeneratePatches(
		t,
		generatorFunc,
		capitest.PatchTestDef{
			Name: "AMI set for control plane",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.AMISpec{ID: "ami-controlplane"},
					variablePath...,
				),
				capitest.VariableWithValue(
					"builtin",
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
					},
				),
			},
			RequestItem: newCPAWSMachineTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/ami/id",
				ValueMatcher: gomega.Equal("ami-controlplane"),
			}},
		},
	)
}

func newCPAWSMachineTemplateRequestItem(
	uid types.UID,
) runtimehooksv1.GeneratePatchesRequestItem {
	return request.NewRequestItem(
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": capav1.GroupVersion.String(),
				"kind":       "AWSMachineTemplate",
				"metadata": map[string]interface{}{
					"name":      "aws-machine-template",
					"namespace": "aws-cluster",
				},
			},
		},
		&runtimehooksv1.HolderReference{
			APIVersion: controlplanev1.GroupVersion.String(),
			Kind:       "KubeadmControlPlane",
			FieldPath:  "spec.machineTemplate.infrastructureRef",
		},
		uid,
	)
}
