// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package golden

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	dockerclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/clusterconfig"
	dockerworkerconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/workerconfig"
)

func TestBuildRequest(t *testing.T) {
	t.Parallel()

	in, err := LoadInput("testdata/corpus/docker/docker-cluster-cilium-helm-addon.yaml")
	require.NoError(t, err)
	in.ClusterClass.Status.Variables = []clusterv1.ClusterClassStatusVariable{
		{
			Name:        "clusterConfig",
			Definitions: []clusterv1.ClusterClassStatusVariableDefinition{{From: "cluster-config"}},
		},
		{Name: "workerConfig", Definitions: []clusterv1.ClusterClassStatusVariableDefinition{{From: "worker-config"}}},
	}

	req, err := BuildRequest(in, "cluster-config")
	require.NoError(t, err)

	type holder struct{ Kind, Name, FieldPath, Template string }
	var holders []holder
	for _, item := range req.Items {
		var tpl struct{ Kind string }
		require.NoError(t, json.Unmarshal(item.Object.Raw, &tpl))
		holders = append(holders, holder{
			item.HolderReference.Kind, item.HolderReference.Name, item.HolderReference.FieldPath, tpl.Kind,
		})
	}
	assert.Equal(t, []holder{
		{"Cluster", "golden", "spec.infrastructureRef", "DockerClusterTemplate"},
		{"Cluster", "golden", "spec.controlPlaneRef", "KubeadmControlPlaneTemplate"},
		{"KubeadmControlPlane", "golden", "spec.machineTemplate.spec.infrastructureRef", "DockerMachineTemplate"},
		{"MachineDeployment", "golden-md-0", "spec.template.spec.bootstrap.configRef", "KubeadmConfigTemplate"},
		{"MachineDeployment", "golden-md-0", "spec.template.spec.infrastructureRef", "DockerMachineTemplate"},
	}, holders)

	assert.Equal(t, []string{"clusterConfig", "builtin"}, variableNames(req.Variables))

	cp := builtins(t, req.Items[1].Variables)
	require.NotNil(t, cp.ControlPlane)
	assert.Equal(t, "v1.35.3", cp.ControlPlane.Version)

	md := builtins(t, req.Items[3].Variables)
	require.NotNil(t, md.MachineDeployment)
	assert.Equal(t, "default-worker", md.MachineDeployment.Class)
	assert.Equal(t, "golden-md-0", md.MachineDeployment.Name)
	assert.Equal(t, "md-0", md.MachineDeployment.TopologyName)
	assert.Equal(t, "v1.35.3", md.MachineDeployment.Version)

	req, err = BuildRequest(in, "worker-config")
	require.NoError(t, err)
	assert.Equal(t, []string{"workerConfig", "builtin"}, variableNames(req.Variables))
}

func TestDefaultVariables(t *testing.T) {
	t.Parallel()

	discovery := map[string]mutation.DiscoverVariables{
		"cluster-config": dockerclusterconfig.NewVariable(),
		"worker-config":  dockerworkerconfig.NewVariable(),
	}

	in, err := LoadInput("testdata/corpus/docker/docker-cluster-cilium-helm-addon.yaml")
	require.NoError(t, err)
	errs, err := DefaultVariables(t.Context(), in, discovery)
	require.NoError(t, err)
	require.Empty(t, errs)

	var clusterConfig struct {
		Addons struct {
			CNI struct{ Strategy string }
		}
		ControlPlane struct {
			NodeRegistration struct{ IgnorePreflightErrors []string }
		}
	}
	require.NoError(t, json.Unmarshal(in.Cluster.Spec.Topology.Variables[0].Value.Raw, &clusterConfig))
	assert.Equal(t, "HelmAddon", clusterConfig.Addons.CNI.Strategy)
	assert.Equal(t, []string{"SystemVerification"}, clusterConfig.ControlPlane.NodeRegistration.IgnorePreflightErrors)

	in, err = LoadInput("testdata/corpus/docker/minimal.yaml")
	require.NoError(t, err)
	in.Cluster.Spec.Topology.Variables[0].Value.Raw = []byte(`{"addons":{"cni":{"provider":"Bogus"}}}`)
	errs, err = DefaultVariables(t.Context(), in, discovery)
	require.NoError(t, err)
	assert.NotEmpty(t, errs)
}

type setField struct{}

func (setField) Mutate(
	_ context.Context,
	obj *unstructured.Unstructured,
	_ map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ client.ObjectKey,
	_ mutation.ClusterGetter,
) error {
	if holderRef.FieldPath != "spec.infrastructureRef" {
		return nil
	}
	return unstructured.SetNestedField(obj.Object, "set", "spec", "template", "spec", "goldenTest")
}

func TestRun(t *testing.T) {
	t.Parallel()

	in, err := LoadInput("testdata/corpus/docker/docker-cluster-cilium-helm-addon.yaml")
	require.NoError(t, err)
	h := mutation.NewMetaGeneratePatchesHandler("test", testEnv.Client, setField{})
	out, err := Run(t.Context(), testEnv.Client, h.(mutation.GeneratePatches), in, "cluster-config",
		map[string]mutation.DiscoverVariables{
			"cluster-config": dockerclusterconfig.NewVariable(),
			"worker-config":  dockerworkerconfig.NewVariable(),
		})
	require.NoError(t, err)

	var items []struct {
		HolderRef runtimehooksv1.HolderReference `json:"holderRef"`
		Object    map[string]any                 `json:"object"`
	}
	require.NoError(t, yaml.Unmarshal(out, &items))
	require.Len(t, items, 5)

	assert.Equal(t, Namespace, items[0].HolderRef.Namespace)
	value, _, _ := unstructured.NestedString(items[0].Object, "spec", "template", "spec", "goldenTest")
	assert.Equal(t, "set", value)

	unchanged := in.Templates[TemplateKey{Kind: "KubeadmControlPlaneTemplate", Name: "docker-quick-start-control-plane"}]
	unchanged = unchanged.DeepCopy()
	unchanged.SetNamespace(Namespace)
	assert.Equal(t, unchanged.Object, items[1].Object)
}

func variableNames(vars []runtimehooksv1.Variable) []string {
	var names []string
	for _, v := range vars {
		names = append(names, v.Name)
	}
	return names
}

func builtins(t *testing.T, vars []runtimehooksv1.Variable) runtimehooksv1.Builtins {
	t.Helper()
	for _, v := range vars {
		if v.Name == runtimehooksv1.BuiltinsName {
			var b runtimehooksv1.Builtins
			require.NoError(t, json.Unmarshal(v.Value.Raw, &b))
			return b
		}
	}
	t.Fatal("no builtin variable")
	return runtimehooksv1.Builtins{}
}

func TestLoadInput(t *testing.T) {
	t.Parallel()

	in, err := LoadInput("testdata/corpus/docker/docker-cluster-cilium-helm-addon.yaml")
	require.NoError(t, err)
	assert.Equal(t, "docker-quick-start", in.ClusterClass.Name)
	assert.Equal(t, "golden", in.Cluster.Name)
	assert.Len(t, in.Templates, 5)
	assert.Contains(t, in.Templates, TemplateKey{Kind: "DockerClusterTemplate", Name: "docker-quick-start-cluster"})
	assert.Len(t, in.Fixtures, 1, "the shared registry addon CA fixture")

	in, err = LoadInput("testdata/corpus/nutanix/nutanix-cluster-cilium-helm-addon.yaml")
	require.NoError(t, err)
	assert.Len(t, in.Templates, 5)
	assert.Len(t, in.Fixtures, 5, "four example Secrets and the shared fixture")
}
