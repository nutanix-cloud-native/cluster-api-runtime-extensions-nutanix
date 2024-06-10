// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package machinedetails

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
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
			log.V(5).Info("Nutanix machine details variable for workers not defined")
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

	return patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		h.patchSelector,
		log,
		func(obj *capxv1.NutanixMachineTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("setting Nutanix machine details in worker NutanixMachineTemplate spec")

			spec := obj.Spec.Template.Spec

			spec.BootType = nutanixMachineDetailsVar.BootType
			spec.Cluster = nutanixMachineDetailsVar.Cluster
			spec.Image = nutanixMachineDetailsVar.Image

			spec.VCPUSockets = nutanixMachineDetailsVar.VCPUSockets
			spec.VCPUsPerSocket = nutanixMachineDetailsVar.VCPUsPerSocket
			spec.MemorySize = nutanixMachineDetailsVar.MemorySize
			spec.SystemDiskSize = nutanixMachineDetailsVar.SystemDiskSize

			spec.Subnets = make(
				[]capxv1.NutanixResourceIdentifier,
				len(nutanixMachineDetailsVar.Subnets),
			)

			copy(spec.Subnets, nutanixMachineDetailsVar.Subnets)

			spec.AdditionalCategories = make(
				[]capxv1.NutanixCategoryIdentifier,
				len(nutanixMachineDetailsVar.AdditionalCategories),
			)

			copy(spec.AdditionalCategories, nutanixMachineDetailsVar.AdditionalCategories)

			if nutanixMachineDetailsVar.Project != nil {
				spec.Project = ptr.To(
					*nutanixMachineDetailsVar.Project,
				)
			}
			spec.GPUs = make(
				[]capxv1.NutanixGPU,
				len(nutanixMachineDetailsVar.GPUs),
			)
			copy(spec.GPUs, nutanixMachineDetailsVar.GPUs)
			obj.Spec.Template.Spec = spec
			return nil
		},
	)
}
