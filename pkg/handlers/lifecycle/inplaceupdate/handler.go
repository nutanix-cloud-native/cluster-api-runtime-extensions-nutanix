// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package inplaceupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta1"
	bootstrapv2 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	nodemanagerv1alpha1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/node-manager/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	commonhandlers "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
)

const (
	// Label key for the machine name used to find/create NodeTasks for in-place update.
	inPlaceUpdateMachineLabelKey = "inplaceupdate.cluster.x-k8s.io/machine"
	// Retry interval when NodeTask is still in progress.
	retryAfterSecondsInProgress = 5
	// Paths we update in-place (credential-provider and containerd config).
	pathImageCredentialProviderConfig   = "/etc/kubernetes/image-credential-provider-config.yaml"
	pathDynamicCredentialProviderConfig = "/etc/kubernetes/dynamic-credential-provider-config.yaml"
	pathStaticImageCredentials          = "/etc/kubernetes/static-image-credentials.json"
	pathContainerdCACert                = "/etc/containerd/certs.d/registry:5000/ca.crt"
	pathRegistryConfigToml              = "/etc/caren/containerd/patches/registry-config.toml"
	containerdRestartScript             = "/bin/bash /etc/caren/containerd/restart.sh"
	defaultFileOwner                    = "root:root"
	defaultFilePermissions              = "0600"
)

// Handler updates credential-provider and containerd config on the node via NodeTask
// when KubeadmConfigTemplate changes; requires Node name to match Machine name.
type Handler struct {
	client ctrlclient.Client
}

var (
	_ commonhandlers.Named          = &Handler{}
	_ lifecycle.CanUpdateMachine    = &Handler{}
	_ lifecycle.CanUpdateMachineSet = &Handler{}
	_ lifecycle.UpdateMachine       = &Handler{}
)

func New(client ctrlclient.Client) *Handler {
	return &Handler{client: client}
}

func (h *Handler) Name() string {
	return "InPlaceUpdateHandler"
}

// getBootstrapKubeadmAPIVersion returns "v1beta2" if raw has apiVersion containing v1beta2, else "v1beta1".
func getBootstrapKubeadmAPIVersion(raw []byte) string {
	if len(raw) == 0 {
		return "v1beta1"
	}
	var m struct {
		APIVersion string `json:"apiVersion"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return "v1beta1"
	}
	if strings.Contains(m.APIVersion, "v1beta2") {
		return "v1beta2"
	}
	return "v1beta1"
}

// normalizeKubeadmBootstrapRaw converts kubeletExtraArgs/extraArgs from array to map form so
// raw can be decoded into v1beta1 types (which use map[string]string). Only use when decoding as v1beta1.
func normalizeKubeadmBootstrapRaw(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	normalizeExtraArgsMaps(m)
	return json.Marshal(m)
}

var extraArgsKeys = map[string]struct{}{"kubeletExtraArgs": {}, "extraArgs": {}}

func normalizeExtraArgsMaps(m map[string]interface{}) {
	for k, v := range m {
		if _, ok := extraArgsKeys[k]; ok {
			if arr, ok := v.([]interface{}); ok {
				if converted := extraArgsArrayToMap(arr); converted != nil {
					m[k] = converted
				}
			}
			continue
		}
		if nested, ok := v.(map[string]interface{}); ok {
			normalizeExtraArgsMaps(nested)
		}
	}
}

func extraArgsArrayToMap(arr []interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(arr))
	for _, item := range arr {
		var key, val string
		switch t := item.(type) {
		case map[string]interface{}:
			if n, ok := t["name"]; ok {
				key = fmt.Sprint(n)
			}
			if v, ok := t["value"]; ok {
				val = fmt.Sprint(v)
			}
		case []interface{}:
			if len(t) >= 2 {
				key = fmt.Sprint(t[0])
				val = fmt.Sprint(t[1])
			}
		case string:
			if idx := strings.IndexByte(t, '='); idx >= 0 {
				key = t[:idx]
				val = t[idx+1:]
			}
		}
		if key != "" {
			out[key] = val
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// specFromBootstrapRaw parses raw KubeadmConfig JSON and returns the top-level "spec" as map. Version-agnostic.
func specFromBootstrapRaw(raw []byte) (map[string]interface{}, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	spec, _ := obj["spec"].(map[string]interface{})
	if spec == nil {
		return nil, fmt.Errorf("missing or invalid spec")
	}
	return spec, nil
}

// specFromTemplateRaw parses raw KubeadmConfigTemplate JSON and returns the top-level "spec" as map. Version-agnostic.
func specFromTemplateRaw(raw []byte) (map[string]interface{}, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	spec, _ := obj["spec"].(map[string]interface{})
	if spec == nil {
		return nil, fmt.Errorf("missing or invalid spec")
	}
	return spec, nil
}

// specsEqual reports whether two spec maps are equal (deep equality).
func specsEqual(a, b map[string]interface{}) bool {
	return reflect.DeepEqual(a, b)
}

// clusterInPlaceUpdateSupported returns true if the Cluster has the in-place update annotation set to "true".
// When false or unset (or Cluster not found), the handler does not set patches so CAPI falls back to rolling out new machines.
func (h *Handler) clusterInPlaceUpdateSupported(ctx context.Context, namespace, clusterName string) bool {
	if clusterName == "" {
		return false
	}
	var cluster clusterv1.Cluster
	if err := h.client.Get(ctx, ctrlclient.ObjectKey{Namespace: namespace, Name: clusterName}, &cluster); err != nil {
		return false
	}
	if cluster.Annotations == nil {
		return false
	}
	return cluster.Annotations[v1alpha1.InPlaceUpdateSupportAnnotationKey] == "true"
}

func (h *Handler) CanUpdateMachine(
	ctx context.Context,
	req *runtimehooksv1.CanUpdateMachineRequest,
	resp *runtimehooksv1.CanUpdateMachineResponse,
) {
	log := ctrl.LoggerFrom(ctx).
		WithValues("hook", "CanUpdateMachine", "machine", req.Desired.Machine.Name, "namespace", req.Desired.Machine.Namespace)
	log.V(1).
		Info("CanUpdateMachine called", "hasCurrentBootstrap", req.Current.BootstrapConfig.Raw != nil, "hasDesiredBootstrap", req.Desired.BootstrapConfig.Raw != nil)

	resp.Status = runtimehooksv1.ResponseStatusSuccess
	resp.Message = "In-place update supported for credential-provider and containerd config"

	clusterName := req.Desired.Machine.Labels[clusterv1.ClusterNameLabel]
	if !h.clusterInPlaceUpdateSupported(ctx, req.Desired.Machine.Namespace, clusterName) {
		log.Info(
			"In-place update not enabled for cluster (annotation not set to true on Cluster), skipping patch; CAPI will roll out new machines",
		)
		return
	}
	if req.Current.BootstrapConfig.Raw == nil || req.Desired.BootstrapConfig.Raw == nil {
		log.Info("No bootstrap config in current or desired, skipping patch")
		return
	}
	currentSpec, err := specFromBootstrapRaw(req.Current.BootstrapConfig.Raw)
	if err != nil {
		log.Error(err, "Failed to parse current bootstrap config")
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = fmt.Sprintf("parse current bootstrap config: %v", err)
		return
	}
	desiredSpec, err := specFromBootstrapRaw(req.Desired.BootstrapConfig.Raw)
	if err != nil {
		log.Error(err, "Failed to parse desired bootstrap config")
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = fmt.Sprintf("parse desired bootstrap config: %v", err)
		return
	}
	if specsEqual(currentSpec, desiredSpec) {
		log.Info("Current and desired bootstrap spec identical, no patch set")
		return
	}
	patch, err := json.Marshal(map[string]interface{}{"spec": desiredSpec})
	if err != nil {
		log.Error(err, "Failed to marshal patch")
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = fmt.Sprintf("marshal patch: %v", err)
		return
	}
	resp.BootstrapConfigPatch = runtimehooksv1.Patch{
		PatchType: runtimehooksv1.JSONMergePatchType,
		Patch:     patch,
	}
	log.Info("CanUpdateMachine returning with BootstrapConfigPatch", "patchLen", len(patch))
}

func (h *Handler) CanUpdateMachineSet(
	ctx context.Context,
	req *runtimehooksv1.CanUpdateMachineSetRequest,
	resp *runtimehooksv1.CanUpdateMachineSetResponse,
) {
	log := ctrl.LoggerFrom(ctx).
		WithValues("hook", "CanUpdateMachineSet", "machineSet", req.Desired.MachineSet.Name, "namespace", req.Desired.MachineSet.Namespace)
	log.V(1).
		Info("CanUpdateMachineSet called", "hasCurrentTemplate", req.Current.BootstrapConfigTemplate.Raw != nil, "hasDesiredTemplate", req.Desired.BootstrapConfigTemplate.Raw != nil)

	resp.Status = runtimehooksv1.ResponseStatusSuccess
	resp.Message = "In-place update supported for credential-provider and containerd config"

	clusterName := req.Desired.MachineSet.Labels[clusterv1.ClusterNameLabel]
	if !h.clusterInPlaceUpdateSupported(ctx, req.Desired.MachineSet.Namespace, clusterName) {
		log.Info(
			"In-place update not enabled for cluster (annotation not set to true on Cluster), skipping patch; CAPI will roll out new machines",
		)
		return
	}
	if req.Current.BootstrapConfigTemplate.Raw == nil || req.Desired.BootstrapConfigTemplate.Raw == nil {
		log.Info("No bootstrap config template in current or desired, skipping patch")
		return
	}
	currentSpec, err := specFromTemplateRaw(req.Current.BootstrapConfigTemplate.Raw)
	if err != nil {
		log.Error(err, "Failed to parse current bootstrap config template")
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = fmt.Sprintf("parse current bootstrap config template: %v", err)
		return
	}
	desiredSpec, err := specFromTemplateRaw(req.Desired.BootstrapConfigTemplate.Raw)
	if err != nil {
		log.Error(err, "Failed to parse desired bootstrap config template")
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = fmt.Sprintf("parse desired bootstrap config template: %v", err)
		return
	}
	if specsEqual(currentSpec, desiredSpec) {
		log.Info("Current and desired bootstrap template spec identical, no patch set")
		return
	}
	patch, err := json.Marshal(map[string]interface{}{"spec": desiredSpec})
	if err != nil {
		log.Error(err, "Failed to marshal patch")
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = fmt.Sprintf("marshal patch: %v", err)
		return
	}
	resp.BootstrapConfigTemplatePatch = runtimehooksv1.Patch{
		PatchType: runtimehooksv1.JSONMergePatchType,
		Patch:     patch,
	}
	log.Info("CanUpdateMachineSet returning with BootstrapConfigTemplatePatch", "patchLen", len(patch))
}

func (h *Handler) UpdateMachine(
	ctx context.Context,
	req *runtimehooksv1.UpdateMachineRequest,
	resp *runtimehooksv1.UpdateMachineResponse,
) {
	namespace := req.Desired.Machine.Namespace
	machineName := req.Desired.Machine.Name
	clusterName := req.Desired.Machine.Labels[clusterv1.ClusterNameLabel]
	log := ctrl.LoggerFrom(ctx).
		WithValues("hook", "UpdateMachine", "machine", machineName, "namespace", namespace, "cluster", clusterName)

	log.V(1).
		Info("UpdateMachine called", "hasDesiredBootstrap", req.Desired.BootstrapConfig.Raw != nil, "inPlaceSupport", h.clusterInPlaceUpdateSupported(ctx, namespace, clusterName))

	if !h.clusterInPlaceUpdateSupported(ctx, namespace, clusterName) {
		log.Info(
			"In-place update not enabled for cluster (annotation not set to true on Cluster), refusing to update; CAPI will roll out new machines",
		)
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = "in-place update not enabled: set annotation " + v1alpha1.InPlaceUpdateSupportAnnotationKey + "=true on the Cluster to use in-place update"
		return
	}
	if req.Desired.BootstrapConfig.Raw == nil {
		log.Info("No desired bootstrap config")
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = "missing desired bootstrap config"
		return
	}
	desiredVer := getBootstrapKubeadmAPIVersion(req.Desired.BootstrapConfig.Raw)
	var files []nodemanagerv1alpha1.FileSpec
	if desiredVer == "v1beta2" {
		var desiredBootstrap bootstrapv2.KubeadmConfig
		if err := json.Unmarshal(req.Desired.BootstrapConfig.Raw, &desiredBootstrap); err != nil {
			log.Error(err, "Failed to decode desired bootstrap config (v1beta2)")
			resp.Status = runtimehooksv1.ResponseStatusFailure
			resp.Message = fmt.Sprintf("decode desired bootstrap config: %v", err)
			return
		}
		log.V(1).Info("Decoded desired bootstrap config (v1beta2)", "totalFiles", len(desiredBootstrap.Spec.Files))
		files = bootstrapSpecToNodeTaskFilesV2(desiredBootstrap.Spec)
	} else {
		desiredRaw, err := normalizeKubeadmBootstrapRaw(req.Desired.BootstrapConfig.Raw)
		if err != nil {
			log.Error(err, "Failed to normalize desired bootstrap config")
			resp.Status = runtimehooksv1.ResponseStatusFailure
			resp.Message = fmt.Sprintf("normalize desired bootstrap config: %v", err)
			return
		}
		var desiredBootstrap bootstrapv1.KubeadmConfig
		if err := json.Unmarshal(desiredRaw, &desiredBootstrap); err != nil {
			log.Error(err, "Failed to decode desired bootstrap config (v1beta1)")
			resp.Status = runtimehooksv1.ResponseStatusFailure
			resp.Message = fmt.Sprintf("decode desired bootstrap config: %v", err)
			return
		}
		log.V(1).Info("Decoded desired bootstrap config (v1beta1)", "totalFiles", len(desiredBootstrap.Spec.Files))
		files = bootstrapSpecToNodeTaskFilesV1(desiredBootstrap.Spec)
	}

	// Idempotency: list existing NodeTask for this machine by labels.
	labels := map[string]string{inPlaceUpdateMachineLabelKey: machineName}
	if clusterName != "" {
		labels[clusterv1.ClusterNameLabel] = clusterName
	}
	var existingList nodemanagerv1alpha1.NodeTaskList
	if err := h.client.List(ctx, &existingList, ctrlclient.InNamespace(namespace), ctrlclient.MatchingLabels(labels)); err != nil {
		log.Error(err, "Failed to list NodeTasks")
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = fmt.Sprintf("list NodeTasks: %v", err)
		return
	}
	log.V(1).Info("Listed NodeTasks for machine", "count", len(existingList.Items), "labels", labels)

	if len(existingList.Items) > 0 {
		nt := &existingList.Items[0]
		log.Info("Found existing NodeTask", "nodeTask", nt.Name, "phase", nt.Status.Phase)
		switch nt.Status.Phase {
		case nodemanagerv1alpha1.NodeTaskPhaseSucceeded:
			resp.Status = runtimehooksv1.ResponseStatusSuccess
			resp.Message = "in-place update complete"
			resp.SetRetryAfterSeconds(0)
			log.Info("NodeTask succeeded, update complete", "nodeTask", nt.Name)
			return
		case nodemanagerv1alpha1.NodeTaskPhaseFailed, nodemanagerv1alpha1.NodeTaskPhasePartiallyFailed:
			resp.Status = runtimehooksv1.ResponseStatusFailure
			resp.Message = fmt.Sprintf("NodeTask %s phase %s", nt.Name, nt.Status.Phase)
			if len(nt.Status.NodeStatuses) > 0 && nt.Status.NodeStatuses[0].Error != "" {
				resp.Message = nt.Status.NodeStatuses[0].Error
				log.Info(
					"NodeTask failed",
					"nodeTask",
					nt.Name,
					"phase",
					nt.Status.Phase,
					"error",
					nt.Status.NodeStatuses[0].Error,
				)
			}
			return
		default:
			resp.Status = runtimehooksv1.ResponseStatusSuccess
			resp.SetRetryAfterSeconds(retryAfterSecondsInProgress)
			log.Info(
				"NodeTask in progress, will retry",
				"nodeTask",
				nt.Name,
				"phase",
				nt.Status.Phase,
				"retryAfterSeconds",
				retryAfterSecondsInProgress,
			)
			return
		}
	}

	// Build and create NodeTask from desired bootstrap spec.
	log.V(1).
		Info("Built NodeTask file list from bootstrap spec", "inPlaceFileCount", len(files), "paths", filePathsFromSpecs(files))
	if len(files) == 0 {
		log.Info("No supported in-place files in desired bootstrap config")
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = "no supported files to update in desired bootstrap config"
		return
	}
	log.Info("Creating NodeTask", "nodeTaskName", "inplace-"+machineName, "fileCount", len(files))
	nodeTask := &nodemanagerv1alpha1.NodeTask{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "inplace-" + machineName,
			Labels: map[string]string{
				inPlaceUpdateMachineLabelKey: machineName,
			},
		},
		Spec: nodemanagerv1alpha1.NodeTaskSpec{
			NodeSelector: &nodemanagerv1alpha1.NodeSelector{
				NodeNames: []string{machineName},
			},
			Actions: []nodemanagerv1alpha1.Action{
				{
					Type:  "WriteFile",
					Name:  "write-credential-and-containerd-files",
					Files: files,
				},
				{
					Type:     "ExecuteCommand",
					Name:     "restart-containerd",
					Commands: []string{containerdRestartScript},
					Timeout:  &metav1.Duration{Duration: 120 * time.Second},
				},
			},
		},
	}
	if clusterName != "" {
		nodeTask.Labels[clusterv1.ClusterNameLabel] = clusterName
	}
	if err := h.client.Create(ctx, nodeTask); err != nil {
		log.Error(err, "Failed to create NodeTask")
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = fmt.Sprintf("create NodeTask: %v", err)
		return
	}
	log.Info(
		"Created NodeTask, will retry until complete",
		"nodeTask",
		nodeTask.Name,
		"retryAfterSeconds",
		retryAfterSecondsInProgress,
	)
	resp.Status = runtimehooksv1.ResponseStatusSuccess
	resp.SetRetryAfterSeconds(retryAfterSecondsInProgress)
}

// filePathsFromSpecs returns a slice of paths from FileSpecs for logging.
func filePathsFromSpecs(specs []nodemanagerv1alpha1.FileSpec) []string {
	out := make([]string, 0, len(specs))
	for _, s := range specs {
		out = append(out, s.Path)
	}
	return out
}

// inPlaceUpdatePaths is the set of file paths we update in-place (credential-provider and containerd).
var inPlaceUpdatePaths = map[string]struct{}{
	pathImageCredentialProviderConfig:   {},
	pathDynamicCredentialProviderConfig: {},
	pathStaticImageCredentials:          {},
	pathContainerdCACert:                {},
	pathRegistryConfigToml:              {},
}

// bootstrapSpecToNodeTaskFilesV1 maps v1beta1 spec.files to NodeTask FileSpecs for in-place paths only.
func bootstrapSpecToNodeTaskFilesV1(spec bootstrapv1.KubeadmConfigSpec) []nodemanagerv1alpha1.FileSpec {
	var out []nodemanagerv1alpha1.FileSpec
	for i := range spec.Files {
		f := &spec.Files[i]
		if _, ok := inPlaceUpdatePaths[f.Path]; !ok {
			continue
		}
		perms := f.Permissions
		if perms == "" {
			perms = defaultFilePermissions
		}
		owner := f.Owner
		if owner == "" {
			owner = defaultFileOwner
		}
		fs := nodemanagerv1alpha1.FileSpec{
			Path:        f.Path,
			Permissions: perms,
			Owner:       owner,
		}
		if f.Content != "" {
			fs.Content = f.Content
		} else if f.ContentFrom != nil && f.ContentFrom.Secret.Name != "" && f.ContentFrom.Secret.Key != "" {
			fs.ContentFrom = &nodemanagerv1alpha1.ContentFrom{
				SecretRef: &nodemanagerv1alpha1.SecretRef{
					Name: f.ContentFrom.Secret.Name,
					Key:  f.ContentFrom.Secret.Key,
				},
			}
		} else {
			continue
		}
		out = append(out, fs)
	}
	return out
}

// bootstrapSpecToNodeTaskFilesV2 maps v1beta2 spec.files to NodeTask FileSpecs for in-place paths only.
func bootstrapSpecToNodeTaskFilesV2(spec bootstrapv2.KubeadmConfigSpec) []nodemanagerv1alpha1.FileSpec {
	var out []nodemanagerv1alpha1.FileSpec
	for i := range spec.Files {
		f := &spec.Files[i]
		if _, ok := inPlaceUpdatePaths[f.Path]; !ok {
			continue
		}
		perms := f.Permissions
		if perms == "" {
			perms = defaultFilePermissions
		}
		owner := f.Owner
		if owner == "" {
			owner = defaultFileOwner
		}
		fs := nodemanagerv1alpha1.FileSpec{
			Path:        f.Path,
			Permissions: perms,
			Owner:       owner,
		}
		if f.Content != "" {
			fs.Content = f.Content
		} else if f.ContentFrom.IsDefined() && f.ContentFrom.Secret.Name != "" && f.ContentFrom.Secret.Key != "" {
			fs.ContentFrom = &nodemanagerv1alpha1.ContentFrom{
				SecretRef: &nodemanagerv1alpha1.SecretRef{
					Name: f.ContentFrom.Secret.Name,
					Key:  f.ContentFrom.Secret.Key,
				},
			}
		} else {
			continue
		}
		out = append(out, fs)
	}
	return out
}
