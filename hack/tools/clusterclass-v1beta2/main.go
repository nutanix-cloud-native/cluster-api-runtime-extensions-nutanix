// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// clusterclass-v1beta2 reads multi-document YAML from stdin (kustomize-built
// cluster class manifests), converts ClusterClass, KubeadmControlPlaneTemplate,
// and KubeadmConfigTemplate to v1beta2 using CAPI's ConvertTo for each kind,
// then writes the result to stdout. Used by hack/examples/sync.sh so that
// make examples.sync produces chart default cluster classes compatible with CAPI 1.12 (v1beta2).
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	bootstrapv1beta1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1beta1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "clusterclass-v1beta2: %v\n", err)
		os.Exit(1)
	}
}

func run(r io.Reader, w io.Writer) error {
	decoder := k8syaml.NewYAMLReader(bufio.NewReader(r))
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	defer enc.Close()

	for {
		doc, err := decoder.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read doc: %w", err)
		}
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}

		var obj map[string]interface{}
		if err := yaml.Unmarshal(doc, &obj); err != nil {
			return fmt.Errorf("unmarshal doc: %w", err)
		}

		converted, err := convertDoc(obj)
		if err != nil {
			return err
		}
		if err := enc.Encode(converted); err != nil {
			return fmt.Errorf("encode doc: %w", err)
		}
	}

	_, err := io.Copy(w, &out)
	return err
}

func convertDoc(obj map[string]interface{}) (map[string]interface{}, error) {
	kind, _ := obj["kind"].(string)
	apiVersion, _ := obj["apiVersion"].(string)

	// If kind/apiVersion missing (e.g. malformed kustomize output), infer from structure so we can convert and fix.
	if kind == "" {
		kind, apiVersion = inferKindFromStructure(obj)
		if kind != "" {
			obj["kind"] = kind
			obj["apiVersion"] = apiVersion
		}
	}

	switch kind {
	case "ClusterClass":
		out, err := convertWithCAPI(obj, apiVersion, "cluster.x-k8s.io/v1beta1", convertClusterClassCAPI)
		if err != nil {
			return nil, err
		}
		ensureClusterClassDesiredState(out)
		return out, nil
	case "KubeadmControlPlaneTemplate":
		out, err := convertWithCAPI(
			obj,
			apiVersion,
			"controlplane.cluster.x-k8s.io/v1beta1",
			convertKubeadmControlPlaneTemplateCAPI,
		)
		if err != nil {
			return nil, err
		}
		// When input is already v1beta2 we pass through; upstream may still have object-form extraArgs. Normalize so output is always array form.
		normalizeKubeadmExtraArgs(out)
		ensureOutputMeta(out, "KubeadmControlPlaneTemplate", "controlplane.cluster.x-k8s.io/v1beta2")
		return out, nil
	case "KubeadmConfigTemplate":
		out, err := convertWithCAPI(
			obj,
			apiVersion,
			"bootstrap.cluster.x-k8s.io/v1beta1",
			convertKubeadmConfigTemplateCAPI,
		)
		if err != nil {
			return nil, err
		}
		normalizeKubeadmExtraArgs(out)
		ensureOutputMeta(out, "KubeadmConfigTemplate", "bootstrap.cluster.x-k8s.io/v1beta2")
		return out, nil
	default:
		// Leave other kinds (AWSClusterTemplate, DockerMachineTemplate, etc.) unchanged.
		return obj, nil
	}
}

// inferKindFromStructure infers kind and apiVersion from document structure when they are missing.
func inferKindFromStructure(obj map[string]interface{}) (kind, apiVersion string) {
	spec, _ := obj["spec"].(map[string]interface{})
	if spec == nil {
		return "", ""
	}
	// ClusterClass: has controlPlane, infrastructure, and often patches.
	if _, hasCP := spec["controlPlane"]; hasCP {
		if _, hasInfra := spec["infrastructure"]; hasInfra {
			return "ClusterClass", "cluster.x-k8s.io/v1beta2"
		}
	}
	// KubeadmControlPlaneTemplate: spec.template.spec.kubeadmConfigSpec
	if template, _ := spec["template"].(map[string]interface{}); template != nil {
		if tSpec, _ := template["spec"].(map[string]interface{}); tSpec != nil {
			if _, hasKCP := tSpec["kubeadmConfigSpec"]; hasKCP {
				return "KubeadmControlPlaneTemplate", "controlplane.cluster.x-k8s.io/v1beta2"
			}
			// KubeadmConfigTemplate: has joinConfiguration or files (worker bootstrap).
			if _, hasJoin := tSpec["joinConfiguration"]; hasJoin {
				return "KubeadmConfigTemplate", "bootstrap.cluster.x-k8s.io/v1beta2"
			}
			if _, hasFiles := tSpec["files"]; hasFiles {
				return "KubeadmConfigTemplate", "bootstrap.cluster.x-k8s.io/v1beta2"
			}
		}
	}
	return "", ""
}

// ensureOutputMeta sets apiVersion and kind on the object so output is always v1beta2.
func ensureOutputMeta(obj map[string]interface{}, kind, apiVersion string) {
	if obj == nil {
		return
	}
	obj["apiVersion"] = apiVersion
	obj["kind"] = kind
}

// ensureClusterClassDesiredState ensures ClusterClass output has v1beta2 apiVersion/kind,
// templateRefs for control plane and bootstrap point to v1beta2 (so they match the converted templates),
// and every patch with discoverVariablesExtension also has generatePatchesExtension (required by v1beta2 validation).
func ensureClusterClassDesiredState(obj map[string]interface{}) {
	if obj == nil {
		return
	}
	ensureOutputMeta(obj, "ClusterClass", "cluster.x-k8s.io/v1beta2")

	spec, _ := obj["spec"].(map[string]interface{})
	if spec == nil {
		return
	}
	// Point control plane and bootstrap templateRefs to v1beta2 so ClusterClass reconciles (observedGeneration set).
	if cp, _ := spec["controlPlane"].(map[string]interface{}); cp != nil {
		if tr, _ := cp["templateRef"].(map[string]interface{}); tr != nil {
			if kind, _ := tr["kind"].(string); kind == "KubeadmControlPlaneTemplate" {
				tr["apiVersion"] = "controlplane.cluster.x-k8s.io/v1beta2"
			}
		}
	}
	if workers, _ := spec["workers"].(map[string]interface{}); workers != nil {
		if mdList, _ := workers["machineDeployments"].([]interface{}); mdList != nil {
			for _, md := range mdList {
				if mdMap, _ := md.(map[string]interface{}); mdMap != nil {
					if bootstrap, _ := mdMap["bootstrap"].(map[string]interface{}); bootstrap != nil {
						if tr, _ := bootstrap["templateRef"].(map[string]interface{}); tr != nil {
							if kind, _ := tr["kind"].(string); kind == "KubeadmConfigTemplate" {
								tr["apiVersion"] = "bootstrap.cluster.x-k8s.io/v1beta2"
							}
						}
					}
				}
			}
		}
	}
	patches, _ := spec["patches"].([]interface{})
	if len(patches) == 0 {
		return
	}
	for _, p := range patches {
		pm, _ := p.(map[string]interface{})
		if pm == nil {
			continue
		}
		ext, _ := pm["external"].(map[string]interface{})
		if ext == nil {
			continue
		}
		dv, _ := ext["discoverVariablesExtension"].(string)
		if dv == "" {
			continue
		}
		if _, has := ext["generatePatchesExtension"]; has {
			continue
		}
		// Derive generatePatchesExtension from discoverVariablesExtension name.
		// e.g. awsclusterconfigvars-dv... -> awsclusterv5configpatch-gp...
		//      awsworkerconfigvars-dv... -> awsworkerv5configpatch-gp...
		gp := deriveGeneratePatchesExtension(dv)
		if gp != "" {
			ext["generatePatchesExtension"] = gp
		}
	}
}

// deriveGeneratePatchesExtension returns the generatePatchesExtension name from a discoverVariablesExtension name.
func deriveGeneratePatchesExtension(discoverVariablesExtension string) string {
	const suffix = ".cluster-api-runtime-extensions-nutanix"
	if !strings.HasSuffix(discoverVariablesExtension, suffix) {
		return ""
	}
	base := discoverVariablesExtension[:len(discoverVariablesExtension)-len(suffix)]
	switch {
	case strings.HasSuffix(base, "clusterconfigvars-dv"):
		return strings.TrimSuffix(base, "clusterconfigvars-dv") + "clusterv5configpatch-gp" + suffix
	case strings.HasSuffix(base, "workerconfigvars-dv"):
		return strings.TrimSuffix(base, "workerconfigvars-dv") + "workerv5configpatch-gp" + suffix
	default:
		return ""
	}
}

// normalizeKubeadmExtraArgs converts extraArgs and kubeletExtraArgs from object form to array form
// in KubeadmControlPlaneTemplate or KubeadmConfigTemplate (so v1beta2 passthrough or post-conversion output is valid).
func normalizeKubeadmExtraArgs(obj map[string]interface{}) {
	spec, _ := obj["spec"].(map[string]interface{})
	if spec == nil {
		return
	}
	template, _ := spec["template"].(map[string]interface{})
	if template == nil {
		return
	}
	innerSpec, _ := template["spec"].(map[string]interface{})
	if innerSpec == nil {
		return
	}
	// KubeadmControlPlaneTemplate has kubeadmConfigSpec; KubeadmConfigTemplate has spec directly.
	if kubeadmConfigSpec, ok := innerSpec["kubeadmConfigSpec"].(map[string]interface{}); ok {
		normalizeKubeadmConfigSpecExtraArgs(kubeadmConfigSpec)
	} else {
		normalizeKubeadmConfigSpecExtraArgs(innerSpec)
	}
}

func normalizeKubeadmConfigSpecExtraArgs(spec map[string]interface{}) {
	if cc, ok := spec["clusterConfiguration"].(map[string]interface{}); ok {
		for _, key := range []string{"apiServer", "controllerManager", "scheduler"} {
			if m, ok := cc[key].(map[string]interface{}); ok {
				convertExtraArgsMapToSlice(m, "extraArgs")
			}
		}
	}
	if initConf, ok := spec["initConfiguration"].(map[string]interface{}); ok {
		if nr, ok := initConf["nodeRegistration"].(map[string]interface{}); ok {
			convertExtraArgsMapToSlice(nr, "kubeletExtraArgs")
		}
	}
	if joinConf, ok := spec["joinConfiguration"].(map[string]interface{}); ok {
		if nr, ok := joinConf["nodeRegistration"].(map[string]interface{}); ok {
			convertExtraArgsMapToSlice(nr, "kubeletExtraArgs")
		}
	}
}

func convertExtraArgsMapToSlice(parent map[string]interface{}, key string) {
	v := parent[key]
	if v == nil {
		return
	}
	if _, isSlice := v.([]interface{}); isSlice {
		return
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return
	}
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	slice := make([]interface{}, 0, len(names))
	for _, name := range names {
		slice = append(slice, map[string]interface{}{"name": name, "value": fmt.Sprint(m[name])})
	}
	parent[key] = slice
}

// convertWithCAPI decodes obj (map) into src, calls the given converter to produce dst, then encodes dst back to map.
func convertWithCAPI(
	obj map[string]interface{},
	apiVersion, wantAPIVersion string,
	convert func(srcJSON []byte) ([]byte, error),
) (map[string]interface{}, error) {
	if apiVersion != wantAPIVersion {
		return obj, nil
	}
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	dstBytes, err := convert(jsonBytes)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(dstBytes, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func convertClusterClassCAPI(srcJSON []byte) ([]byte, error) {
	var src clusterv1beta1.ClusterClass
	if err := json.Unmarshal(srcJSON, &src); err != nil {
		return nil, fmt.Errorf("clusterclass unmarshal: %w", err)
	}
	var dst clusterv1.ClusterClass
	if err := src.ConvertTo(&dst); err != nil {
		return nil, fmt.Errorf("clusterclass ConvertTo: %w", err)
	}
	return json.Marshal(&dst)
}

func convertKubeadmControlPlaneTemplateCAPI(srcJSON []byte) ([]byte, error) {
	var src controlplanev1beta1.KubeadmControlPlaneTemplate
	if err := json.Unmarshal(srcJSON, &src); err != nil {
		return nil, fmt.Errorf("kubeadmcontrolplanetemplate unmarshal: %w", err)
	}
	var dst controlplanev1.KubeadmControlPlaneTemplate
	if err := src.ConvertTo(&dst); err != nil {
		return nil, fmt.Errorf("kubeadmcontrolplanetemplate ConvertTo: %w", err)
	}
	return json.Marshal(&dst)
}

func convertKubeadmConfigTemplateCAPI(srcJSON []byte) ([]byte, error) {
	var src bootstrapv1beta1.KubeadmConfigTemplate
	if err := json.Unmarshal(srcJSON, &src); err != nil {
		return nil, fmt.Errorf("kubeadmconfigtemplate unmarshal: %w", err)
	}
	var dst bootstrapv1.KubeadmConfigTemplate
	if err := src.ConvertTo(&dst); err != nil {
		return nil, fmt.Errorf("kubeadmconfigtemplate ConvertTo: %w", err)
	}
	return json.Marshal(&dst)
}
