// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ami

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	VariableName = "ami"
)

type awsAMISpecPatchHandler struct {
	metaVariableName  string
	variableFieldPath []string
	patchSelector     clusterv1.PatchSelector
}

func newAWSAMISpecPatchHandler(
	metaVariableName string,
	variableFieldPath []string,
	patchSelector clusterv1.PatchSelector,
) *awsAMISpecPatchHandler {
	return &awsAMISpecPatchHandler{
		metaVariableName:  metaVariableName,
		variableFieldPath: variableFieldPath,
		patchSelector:     patchSelector,
	}
}

func (h *awsAMISpecPatchHandler) Mutate(
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

	amiSpecVar, err := variables.Get[v1alpha1.AMISpec](
		vars,
		h.metaVariableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).
				Info("AWS amiSpec variable not defined. Default AMI provided by CAPA will be used.")
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
		amiSpecVar,
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
			).Info("setting AMI in AWSMachineTemplate spec")

			if amiSpecVar.ID != "" {
				obj.Spec.Template.Spec.AMI = capav1.AMIReference{ID: &amiSpecVar.ID}
			}
			if amiSpecVar.Lookup != nil {
				obj.Spec.Template.Spec.ImageLookupFormat = amiSpecVar.Lookup.Format
				obj.Spec.Template.Spec.ImageLookupOrg = amiSpecVar.Lookup.Org
				obj.Spec.Template.Spec.ImageLookupBaseOS = amiSpecVar.Lookup.BaseOS
			}

			return nil
		},
	)
}
