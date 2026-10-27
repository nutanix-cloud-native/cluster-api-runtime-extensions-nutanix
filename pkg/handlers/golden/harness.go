// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package golden runs every registered topology mutation handler against a frozen corpus of Clusters and compares the
// result with committed golden files, so that any change to a released handler's output fails the build.
package golden

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
)

// TemplateKey identifies a template referenced by the ClusterClass.
type TemplateKey struct {
	Kind string
	Name string
}

// Input is one corpus file: a ClusterClass with its templates, a Cluster using it, and any objects the handlers read
// from the management cluster.
type Input struct {
	ClusterClass *clusterv1.ClusterClass
	Cluster      *clusterv1.Cluster
	Templates    map[TemplateKey]*unstructured.Unstructured
	Fixtures     []*unstructured.Unstructured
}

func LoadInput(path string) (*Input, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	in := &Input{Templates: map[TemplateKey]*unstructured.Unstructured{}}
	decoder := k8syaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	for {
		u := &unstructured.Unstructured{}
		if err := decoder.Decode(&u.Object); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decoding %s: %w", path, err)
		}
		if len(u.Object) == 0 {
			continue
		}

		switch kind := u.GetKind(); {
		case kind == "ClusterClass":
			in.ClusterClass = &clusterv1.ClusterClass{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, in.ClusterClass)
		case kind == "Cluster":
			in.Cluster = &clusterv1.Cluster{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, in.Cluster)
		case strings.HasSuffix(kind, "Template"):
			in.Templates[TemplateKey{Kind: kind, Name: u.GetName()}] = u
		default:
			in.Fixtures = append(in.Fixtures, u)
		}
		if err != nil {
			return nil, fmt.Errorf("converting %s %s in %s: %w", u.GetKind(), u.GetName(), path, err)
		}
	}

	if in.ClusterClass == nil || in.Cluster == nil {
		return nil, fmt.Errorf("%s must contain one ClusterClass and one Cluster", path)
	}
	return in, nil
}

// BuildRequest builds the GeneratePatchesRequest CAPI sends to the external patch named patch in the ClusterClass,
// following createRequest and addVariablesForPatch in CAPI's internal/controllers/topology/cluster/patches. CAPI
// generates names for the control plane, MachineDeployments and their cloned templates; fixed names are used here so
// that output is deterministic.
func BuildRequest(in *Input, patch string) (*runtimehooksv1.GeneratePatchesRequest, error) {
	cluster, cc := in.Cluster, in.ClusterClass
	topology := cluster.Spec.Topology

	definitions := map[string]bool{}
	for _, v := range cc.Status.Variables {
		for _, d := range v.Definitions {
			if d.From == patch {
				definitions[v.Name] = true
			}
		}
	}
	filter := func(vars []clusterv1.ClusterVariable) []runtimehooksv1.Variable {
		out := []runtimehooksv1.Variable{}
		for _, v := range vars {
			if v.Name != runtimehooksv1.BuiltinsName && definitions[v.Name] {
				out = append(out, runtimehooksv1.Variable{Name: v.Name, Value: v.Value})
			}
		}
		return out
	}

	req := &runtimehooksv1.GeneratePatchesRequest{}
	addItem := func(
		ref clusterv1.ClusterClassTemplateReference,
		holder runtimehooksv1.HolderReference,
		vars []runtimehooksv1.Variable,
	) error {
		tpl, ok := in.Templates[TemplateKey{Kind: ref.Kind, Name: ref.Name}]
		if !ok {
			return fmt.Errorf("ClusterClass references missing template %s %s", ref.Kind, ref.Name)
		}
		tpl = tpl.DeepCopy()
		tpl.SetNamespace(cluster.Namespace)
		raw, err := json.Marshal(tpl)
		if err != nil {
			return err
		}
		holder.Namespace = cluster.Namespace
		req.Items = append(req.Items, runtimehooksv1.GeneratePatchesRequestItem{
			UID:             types.UID(strconv.Itoa(len(req.Items))),
			HolderReference: holder,
			Object:          runtime.RawExtension{Raw: raw},
			Variables:       vars,
		})
		return nil
	}

	global := runtimehooksv1.Builtins{Cluster: &runtimehooksv1.ClusterBuiltins{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
		UID:       cluster.UID,
		Topology: &runtimehooksv1.ClusterTopologyBuiltins{
			Version:        topology.Version,
			Class:          cluster.GetClassKey().Name,
			ClassNamespace: cluster.GetClassKey().Namespace,
			ClassRef: runtimehooksv1.ClusterTopologyClusterClassRefBuiltins{
				Name:      cluster.GetClassKey().Name,
				Namespace: cluster.GetClassKey().Namespace,
			},
		},
		Metadata: metadata(cluster.Labels, cluster.Annotations),
	}}
	if network := cluster.Spec.ClusterNetwork; network.ServiceDomain != "" || network.Services.CIDRBlocks != nil ||
		network.Pods.CIDRBlocks != nil {
		global.Cluster.Network = &runtimehooksv1.ClusterNetworkBuiltins{
			Services: network.Services.CIDRBlocks,
			Pods:     network.Pods.CIDRBlocks,
		}
		if network.ServiceDomain != "" {
			global.Cluster.Network.ServiceDomain = &network.ServiceDomain
		}
	}
	req.Variables = append(filter(topology.Variables), builtinVariable(global))

	clusterHolder := runtimehooksv1.HolderReference{
		APIVersion: clusterv1.GroupVersion.String(),
		Kind:       "Cluster",
		Name:       cluster.Name,
	}
	infraHolder := clusterHolder
	infraHolder.FieldPath = "spec.infrastructureRef"
	if err := addItem(cc.Spec.Infrastructure.TemplateRef, infraHolder, nil); err != nil {
		return nil, err
	}

	cpRef := cc.Spec.ControlPlane.TemplateRef
	cpBuiltin := runtimehooksv1.Builtins{ControlPlane: &runtimehooksv1.ControlPlaneBuiltins{
		Name:    cluster.Name,
		Version: topology.Version,
		Metadata: metadata(
			topologyLabels(cluster, cc.Spec.ControlPlane.Metadata.Labels, topology.ControlPlane.Metadata.Labels),
			merge(cc.Spec.ControlPlane.Metadata.Annotations, topology.ControlPlane.Metadata.Annotations),
		),
	}}
	cpBuiltin.ControlPlane.Replicas = topology.ControlPlane.Replicas
	cpInfraRef := cc.Spec.ControlPlane.MachineInfrastructure.TemplateRef
	if cpInfraRef.Kind != "" {
		cpBuiltin.ControlPlane.MachineTemplate = &runtimehooksv1.ControlPlaneMachineTemplateBuiltins{
			InfrastructureRef: runtimehooksv1.ControlPlaneMachineTemplateInfrastructureRefBuiltins{
				Name: cluster.Name + "-control-plane",
			},
		}
	}
	cpVars := append(filter(topology.ControlPlane.Variables.Overrides), builtinVariable(cpBuiltin))
	cpHolder := clusterHolder
	cpHolder.FieldPath = "spec.controlPlaneRef"
	if err := addItem(cpRef, cpHolder, cpVars); err != nil {
		return nil, err
	}
	if cpInfraRef.Kind != "" {
		holder := runtimehooksv1.HolderReference{
			APIVersion: cpRef.APIVersion,
			Kind:       strings.TrimSuffix(cpRef.Kind, "Template"),
			Name:       cluster.Name,
			FieldPath:  "spec.machineTemplate.spec.infrastructureRef",
		}
		if err := addItem(cpInfraRef, holder, cpVars); err != nil {
			return nil, err
		}
	}

	for i := range topology.Workers.MachineDeployments {
		mdTopology := &topology.Workers.MachineDeployments[i]
		var mdClass *clusterv1.MachineDeploymentClass
		for j := range cc.Spec.Workers.MachineDeployments {
			if cc.Spec.Workers.MachineDeployments[j].Class == mdTopology.Class {
				mdClass = &cc.Spec.Workers.MachineDeployments[j]
			}
		}
		if mdClass == nil {
			return nil, fmt.Errorf("MachineDeployment class %q not found in ClusterClass", mdTopology.Class)
		}

		name := cluster.Name + "-" + mdTopology.Name
		labels := topologyLabels(cluster, mdClass.Metadata.Labels, mdTopology.Metadata.Labels)
		labels[clusterv1.ClusterTopologyMachineDeploymentNameLabel] = mdTopology.Name
		mdBuiltin := runtimehooksv1.Builtins{MachineDeployment: &runtimehooksv1.MachineDeploymentBuiltins{
			Version:      topology.Version,
			Class:        mdTopology.Class,
			Name:         name,
			TopologyName: mdTopology.Name,
			Metadata:     metadata(labels, merge(mdClass.Metadata.Annotations, mdTopology.Metadata.Annotations)),
			Bootstrap: &runtimehooksv1.MachineBootstrapBuiltins{
				ConfigRef: &runtimehooksv1.MachineBootstrapConfigRefBuiltins{Name: name + "-bootstrap"},
			},
			InfrastructureRef: &runtimehooksv1.MachineInfrastructureRefBuiltins{Name: name + "-infra"},
		}}
		if r := mdTopology.Replicas; r != nil {
			mdBuiltin.MachineDeployment.Replicas = ptr.To(int64(*r))
		}
		mdVars := append(filter(mdTopology.Variables.Overrides), builtinVariable(mdBuiltin))

		holder := runtimehooksv1.HolderReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "MachineDeployment",
			Name:       name,
			FieldPath:  "spec.template.spec.bootstrap.configRef",
		}
		if err := addItem(mdClass.Bootstrap.TemplateRef, holder, mdVars); err != nil {
			return nil, err
		}
		holder.FieldPath = "spec.template.spec.infrastructureRef"
		if err := addItem(mdClass.Infrastructure.TemplateRef, holder, mdVars); err != nil {
			return nil, err
		}
	}

	return req, nil
}

func builtinVariable(b runtimehooksv1.Builtins) runtimehooksv1.Variable {
	raw, err := json.Marshal(b)
	if err != nil {
		panic(err) // Builtins only contains marshalable fields.
	}
	return runtimehooksv1.Variable{Name: runtimehooksv1.BuiltinsName, Value: apiextensionsv1.JSON{Raw: raw}}
}

func metadata(labels, annotations map[string]string) *runtimehooksv1.ObjectMeta {
	if labels == nil && annotations == nil {
		return nil
	}
	return &runtimehooksv1.ObjectMeta{Labels: labels, Annotations: annotations}
}

// topologyLabels returns the labels CAPI sets on objects it creates for a topology.
func topologyLabels(cluster *clusterv1.Cluster, fromClass, fromTopology map[string]string) map[string]string {
	labels := merge(fromClass, fromTopology)
	if labels == nil {
		labels = map[string]string{}
	}
	labels[clusterv1.ClusterNameLabel] = cluster.Name
	labels[clusterv1.ClusterTopologyOwnedLabel] = ""
	return labels
}

func merge(maps ...map[string]string) map[string]string {
	var out map[string]string
	for _, m := range maps {
		for k, v := range m {
			if out == nil {
				out = map[string]string{}
			}
			out[k] = v
		}
	}
	return out
}
