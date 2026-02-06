// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package parallelimagepulls

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "maxParallelImagePullsPerNode"

	kubeletConfigurationPatchFilePath = "/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json"
)

var (
	//go:embed embedded/kubeletconfigpatch.yaml
	kubeletConfigPatchYAML []byte

	kubeletConfigPatchTemplate = template.Must(template.New("kubeletConfigPatch").Parse(string(kubeletConfigPatchYAML)))
)

type maxParallelImagePullsPerNode struct {
	variableName      string
	variableFieldPath []string
}

func NewPatch() *maxParallelImagePullsPerNode {
	return newMaxParallelImagePullsPerNodePatch(
		v1alpha1.ClusterConfigVariableName,
		VariableName,
	)
}

func newMaxParallelImagePullsPerNodePatch(
	variableName string,
	variableFieldPath ...string,
) *maxParallelImagePullsPerNode {
	return &maxParallelImagePullsPerNode{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *maxParallelImagePullsPerNode) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ client.ObjectKey,
	clusterGetter mutation.ClusterGetter,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	maxParallelImagePullsPerNode, err := variables.Get[int32](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("max parallel image pulls is not set, skipping mutation")
			return nil
		}
		return err
	}

	if maxParallelImagePullsPerNode == 1 {
		log.V(5).Info("max parallel image pulls is set to 1, skipping mutation resulting in serialized image pulls")
		return nil
	}

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		maxParallelImagePullsPerNode,
	)

	kubeletConfigPatch, err := templateMaxParallelImagePullsPerNodeConfigFile(maxParallelImagePullsPerNode)
	if err != nil {
		return err
	}

	if err := patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.ControlPlane(),
		log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("adding max parallel image pulls patch to control plane kubeadm config spec")

			obj.Spec.Template.Spec.KubeadmConfigSpec.Files = append(
				obj.Spec.Template.Spec.KubeadmConfigSpec.Files,
				*kubeletConfigPatch,
			)

			return nil
		},
	); err != nil {
		return err
	}

	if err := patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.WorkersKubeadmConfigTemplateSelector(),
		log,
		func(obj *bootstrapv1.KubeadmConfigTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("adding max parallel image pulls patch to worker node kubeadm config template")

			obj.Spec.Template.Spec.Files = append(
				obj.Spec.Template.Spec.Files,
				*kubeletConfigPatch,
			)

			return nil
		},
	); err != nil {
		return err
	}

	return nil
}

// templateMaxParallelImagePullsPerNodeConfigFile adds the max parallel image pulls configuration patch file
// to the KCPTemplate.
func templateMaxParallelImagePullsPerNodeConfigFile(
	maxParallelImagePullsPerNode int32,
) (*bootstrapv1.File, error) {
	templateInput := struct {
		MaxParallelImagePullsPerNode int32
	}{
		MaxParallelImagePullsPerNode: maxParallelImagePullsPerNode,
	}
	var b bytes.Buffer
	err := kubeletConfigPatchTemplate.Execute(&b, templateInput)
	if err != nil {
		return nil, fmt.Errorf("failed executing kubeletconfig patch template: %w", err)
	}

	return &bootstrapv1.File{
		Path:        kubeletConfigurationPatchFilePath,
		Owner:       "root:root",
		Permissions: "0644",
		Content:     b.String(),
	}, nil
}
