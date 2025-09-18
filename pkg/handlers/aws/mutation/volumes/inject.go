// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package volumes

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capav1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "volumes"
)

type awsVolumesSpecPatchHandler struct {
	metaVariableName  string
	variableFieldPath []string
	patchSelector     clusterv1.PatchSelector
}

func NewAWSVolumesSpecPatchHandler(
	metaVariableName string,
	variableFieldPath []string,
	patchSelector clusterv1.PatchSelector,
) *awsVolumesSpecPatchHandler {
	return &awsVolumesSpecPatchHandler{
		metaVariableName:  metaVariableName,
		variableFieldPath: variableFieldPath,
		patchSelector:     patchSelector,
	}
}

func (h *awsVolumesSpecPatchHandler) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ client.ObjectKey,
	_ mutation.ClusterGetter,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)
	volumesVar, err := variables.Get[v1alpha1.AWSVolumes](
		vars,
		h.metaVariableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).
				Info("No volumes configuration provided. Skipping.")
			return nil
		}
		return err
	}

	log = log.WithValues(
		"variableName",
		h.metaVariableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		volumesVar,
	)

	return patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		h.patchSelector,
		log,
		func(obj *capav1.AWSMachineTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("setting volumes configuration")

			// Handle root volume
			if volumesVar.Root != nil {
				rootVolume := h.toCAPAVolume(volumesVar.Root)
				obj.Spec.Template.Spec.RootVolume = rootVolume
			}

			// Handle non-root volumes
			if len(volumesVar.NonRoot) > 0 {
				nonRootVolumes := make([]capav1.Volume, 0, len(volumesVar.NonRoot))
				for n := range volumesVar.NonRoot {
					vol := &volumesVar.NonRoot[n]
					nonRootVolumes = append(nonRootVolumes, *h.toCAPAVolume(vol))
				}
				obj.Spec.Template.Spec.NonRootVolumes = nonRootVolumes
			}

			return nil
		},
	)
}

// toCAPAVolume converts v1alpha1.AWSVolume to capav1.Volume.
func (h *awsVolumesSpecPatchHandler) toCAPAVolume(vol *v1alpha1.AWSVolume) *capav1.Volume {
	capav1Volume := &capav1.Volume{
		DeviceName:    vol.DeviceName,
		Size:          vol.Size,
		Type:          vol.Type,
		IOPS:          vol.IOPS,
		EncryptionKey: vol.EncryptionKey,
	}

	// Handle pointer fields - convert non-pointer v1alpha1 fields to pointer capav1 fields
	if vol.Throughput != 0 {
		capav1Volume.Throughput = ptr.To(vol.Throughput)
	}
	if vol.Encrypted {
		capav1Volume.Encrypted = ptr.To(vol.Encrypted)
	}

	return capav1Volume
}
