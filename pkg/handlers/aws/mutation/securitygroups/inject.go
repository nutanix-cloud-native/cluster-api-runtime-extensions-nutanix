// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package securitygroups

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capav1 "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "additionalSecurityGroups"
)

type awsSecurityGroupSpecPatchHandler struct {
	metaVariableName  string
	variableFieldPath []string
	patchSelector     clusterv1.PatchSelector
}

func NewAWSSecurityGroupSpecPatchHandler(
	metaVariableName string,
	variableFieldPath []string,
	patchSelector clusterv1.PatchSelector,
) *awsSecurityGroupSpecPatchHandler {
	return &awsSecurityGroupSpecPatchHandler{
		metaVariableName:  metaVariableName,
		variableFieldPath: variableFieldPath,
		patchSelector:     patchSelector,
	}
}

func (h *awsSecurityGroupSpecPatchHandler) Mutate(
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
	additionalSecGroupVar, err := variables.Get[v1alpha1.AdditionalSecurityGroup](
		vars,
		h.metaVariableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).
				Info("No additional security groups provided. Skipping.")
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
		additionalSecGroupVar,
	)
	resourceRefs := make([]capav1.AWSResourceReference, 0, len(additionalSecGroupVar))
	for _, secGroup := range additionalSecGroupVar {
		resourceRefs = append(resourceRefs, capav1.AWSResourceReference{
			ID: ptr.To(secGroup.ID),
		})
	}
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
			).Info("setting additional security groups ")

			obj.Spec.Template.Spec.AdditionalSecurityGroups = resourceRefs
			return nil
		},
	)
}
