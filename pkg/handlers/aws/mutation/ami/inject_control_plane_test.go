// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ami

import (
	"testing"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/k8s/parser"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest/request"
	awsclusterconfig "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/aws/clusterconfig"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
	"github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
			Name: "AMI set for control plane, empty AWSMachineTemplate input",
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
			RequestItem: newCPAWSMachineTemplateRequestItem(
				"1234",
				`
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind:       "AWSMachineTemplate"
metadata:
  name:      "aws-machine-template"
  namespace: "aws-cluster"
  creationTimestamp: "2023-10-10T10:27:48Z"
`,
			),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec",
				ValueMatcher: gomega.Equal(
					map[string]interface{}{
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{},
							"spec": map[string]interface{}{
								"ami": map[string]interface{}{
									"id": "ami-controlplane",
								},
								"cloudInit":    map[string]interface{}{},
								"instanceType": "",
							},
						},
					},
				),
			}},
		},
		capitest.PatchTestDef{
			Name: "AMI set for control plane, AMI field initialized with no ID",
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
			RequestItem: newCPAWSMachineTemplateRequestItem(
				"1234",
				`
apiVersion: infrastructure.cluster.x-k8s.io/v1beta2
kind:       "AWSMachineTemplate"
metadata:
  name:      "aws-machine-template"
  namespace: "aws-cluster"
  creationTimestamp: "2023-10-10T10:27:48Z"
spec:
  template:
    spec:
      ami: {}
`,
			),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/metadata",
				ValueMatcher: gomega.Equal(map[string]interface{}{}),
			}, {
				Operation:    "add",
				Path:         "/spec/template/spec/ami/id",
				ValueMatcher: gomega.Equal("ami-controlplane"),
			}, {
				Operation:    "add",
				Path:         "/spec/template/spec/cloudInit",
				ValueMatcher: gomega.Equal(map[string]interface{}{}),
			}, {
				Operation:    "add",
				Path:         "/spec/template/spec/instanceType",
				ValueMatcher: gomega.Equal(""),
			}},
		},
	)
}

func newCPAWSMachineTemplateRequestItem(
	uid types.UID,
	awsMachineTemplateJSON string,
) runtimehooksv1.GeneratePatchesRequestItem {
	parsed, err := parser.StringsToUnstructured(awsMachineTemplateJSON)
	if err != nil {
		panic(err)
	}
	return request.NewRequestItem(
		parsed[0],
		&runtimehooksv1.HolderReference{
			APIVersion: controlplanev1.GroupVersion.String(),
			Kind:       "KubeadmControlPlane",
			FieldPath:  "spec.machineTemplate.infrastructureRef",
		},
		uid,
	)
}
