// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nodelabels

import (
	"context"
	"fmt"
	"sort"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

const (
	// VariableName is intentionally empty; this mutation is unconditional for workers.
	VariableName = ""

	// kubeletArgNodeLabels is the kubelet extra arg key controlling node labels.
	kubeletArgNodeLabels = "node-labels"

	// workerRoleLabel is the canonical worker role label.
	workerRoleLabel = "node-role.kubernetes.io/worker"
)

type workerNodeLabelsPatchHandler struct {
	variableName      string
	variableFieldPath []string
}

// NewWorkerPatch returns a mutator that ensures worker nodes get the worker role label.
func NewWorkerPatch() *workerNodeLabelsPatchHandler {
	return &workerNodeLabelsPatchHandler{
		variableName:      v1alpha1.WorkerConfigVariableName,
		variableFieldPath: []string{},
	}
}

func (h *workerNodeLabelsPatchHandler) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ ctrlclient.ObjectKey,
	_ mutation.ClusterGetter,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	// Best-effort fetch of worker variable to keep symmetry with other handlers; ignore not found.
	if _, err := variables.Get[map[string]any](
		vars,
		h.variableName,
		h.variableFieldPath...,
	); err != nil && !variables.IsNotFoundError(err) {
		return err
	}

	return patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.WorkersKubeadmConfigTemplateSelector(), log,
		func(obj *bootstrapv1.KubeadmConfigTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", ctrlclient.ObjectKeyFromObject(obj),
			).Info("ensuring worker node label via kubelet extra args on join")

			if obj.Spec.Template.Spec.JoinConfiguration == nil {
				obj.Spec.Template.Spec.JoinConfiguration = &bootstrapv1.JoinConfiguration{}
			}

			args := obj.Spec.Template.Spec.JoinConfiguration.NodeRegistration.KubeletExtraArgs
			if args == nil {
				args = map[string]string{}
			}

			updated := ensureWorkerNodeLabel(args)
			if updated {
				obj.Spec.Template.Spec.JoinConfiguration.NodeRegistration.KubeletExtraArgs = args
			}

			return nil
		},
	)
}

// ensureWorkerNodeLabel adds workerRoleLabel to kubeletArgNodeLabels if not already present.
// It returns true if args were updated.
func ensureWorkerNodeLabel(args map[string]string) bool {
	// Build set of labels from existing arg, if any.
	existing := args[kubeletArgNodeLabels]
	if existing == "" {
		args[kubeletArgNodeLabels] = workerRoleLabel + "="
		return true
	}

	// Parse comma-separated list of key[=value] entries.
	parts := strings.Split(existing, ",")
	// Track presence and rebuild normalized list.
	hasWorker := false
	normalized := make([]string, 0, len(parts)+1)
	seen := make(map[string]struct{})

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		key := p
		if idx := strings.IndexByte(p, '='); idx >= 0 {
			key = p[:idx]
		}
		if key == workerRoleLabel {
			hasWorker = true
			// Normalize to key=
			p = fmt.Sprintf("%s=", workerRoleLabel)
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		normalized = append(normalized, p)
	}

	if !hasWorker {
		normalized = append(normalized, workerRoleLabel+"=")
	}

	sort.Strings(normalized)
	newVal := strings.Join(normalized, ",")
	if newVal == existing {
		return false
	}
	args[kubeletArgNodeLabels] = newVal
	return true
}
