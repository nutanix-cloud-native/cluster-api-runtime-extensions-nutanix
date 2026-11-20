// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package machinedetails

import (
	"context"
	"errors"
	"slices"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "machineDetails"
)

type nutanixMachineDetailsPatchHandler struct {
	metaVariableName  string
	variableFieldPath []string
	patchSelector     clusterv1.PatchSelector
}

func removeVMProfileConflictingFieldsFromNutanixMachineTemplate(u *unstructured.Unstructured) {
	unstructured.RemoveNestedField(u.Object, "spec", "template", "spec", "vcpuSockets")
	unstructured.RemoveNestedField(u.Object, "spec", "template", "spec", "vcpusPerSocket")
	unstructured.RemoveNestedField(u.Object, "spec", "template", "spec", "memorySize")
	unstructured.RemoveNestedField(u.Object, "spec", "template", "spec", "bootType")
	unstructured.RemoveNestedField(u.Object, "spec", "template", "spec", "gpus")
}

// ErrNoImageOrImageLookupSet is returned when neither image nor imageLookup is set in the resulting template spec.
var ErrNoImageOrImageLookupSet = errors.New("image or image lookup must be set")

func newNutanixMachineDetailsPatchHandler(
	metaVariableName string,
	variableFieldPath []string,
	patchSelector clusterv1.PatchSelector,
) *nutanixMachineDetailsPatchHandler {
	return &nutanixMachineDetailsPatchHandler{
		metaVariableName:  metaVariableName,
		variableFieldPath: variableFieldPath,
		patchSelector:     patchSelector,
	}
}

func (h *nutanixMachineDetailsPatchHandler) Mutate(
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

	nutanixMachineDetailsVar, err := variables.Get[v1alpha1.NutanixMachineDetails](
		vars,
		h.metaVariableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("Nutanix machine details variable not defined, skipping patch")
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
		nutanixMachineDetailsVar,
	)

	if err := patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		h.patchSelector,
		log,
		func(obj *capxv1.NutanixMachineTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("setting Nutanix machine details in NutanixMachineTemplate spec")

			spec := obj.Spec.Template.Spec

			if nutanixMachineDetailsVar.Cluster != nil {
				spec.Cluster = *nutanixMachineDetailsVar.Cluster
			}

			if len(nutanixMachineDetailsVar.Subnets) > 0 {
				spec.Subnets = slices.Clone(nutanixMachineDetailsVar.Subnets)
			}

			// If the variable specifies image/imageLookup, it overrides the template.
			// Otherwise, keep the existing template value (ClusterClass templates often define it).
			switch {
			case nutanixMachineDetailsVar.Image != nil:
				spec.Image = nutanixMachineDetailsVar.Image.DeepCopy()
				spec.ImageLookup = nil
			case nutanixMachineDetailsVar.ImageLookup != nil:
				spec.ImageLookup = nutanixMachineDetailsVar.ImageLookup.DeepCopy()
				spec.Image = nil
			default:
				return ErrNoImageOrImageLookupSet
			}

			// VMProfile provides CPU, memory, and bootType configuration.
			// When VMProfile is set, these fields must be omitted from the CAPX spec.
			if nutanixMachineDetailsVar.VMProfile == nil {
				spec.VMProfile = nil
				spec.BootType = nutanixMachineDetailsVar.BootType
				spec.VCPUSockets = nutanixMachineDetailsVar.VCPUSockets
				spec.VCPUsPerSocket = nutanixMachineDetailsVar.VCPUsPerSocket
				spec.MemorySize = nutanixMachineDetailsVar.MemorySize
				spec.GPUs = slices.Clone(nutanixMachineDetailsVar.GPUs)
			} else {
				spec.VMProfile = nutanixMachineDetailsVar.VMProfile.DeepCopy()
			}

			spec.SystemDiskSize = nutanixMachineDetailsVar.SystemDiskSize
			spec.AdditionalCategories = slices.Clone(nutanixMachineDetailsVar.AdditionalCategories)
			spec.Project = nutanixMachineDetailsVar.Project.DeepCopy()

			obj.Spec.Template.Spec = spec
			return nil
		},
	); err != nil {
		return err
	}

	// Force-remove mutually exclusive fields when VMProfile is set.
	//
	// This ensures we emit explicit JSONPatch remove operations for fields that may be present in the base
	// templates (e.g. the default Nutanix ClusterClass templates), satisfying CAPX's CEL validations.
	if nutanixMachineDetailsVar.VMProfile != nil {
		removeVMProfileConflictingFieldsFromNutanixMachineTemplate(obj)
	}

	return nil
}
