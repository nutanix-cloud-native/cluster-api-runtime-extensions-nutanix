// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package failuredomain

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "failureDomain"
)

type awsFailureDomainWorkerPatchHandler struct {
	variableName      string
	variableFieldPath []string
}

func NewWorkerPatch() *awsFailureDomainWorkerPatchHandler {
	return NewAWSFailureDomainWorkerPatchHandler(
		v1alpha1.WorkerConfigVariableName,
		v1alpha1.AWSVariableName,
		VariableName,
	)
}

func NewAWSFailureDomainWorkerPatchHandler(
	variableName string,
	variableFieldPath ...string,
) *awsFailureDomainWorkerPatchHandler {
	return &awsFailureDomainWorkerPatchHandler{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *awsFailureDomainWorkerPatchHandler) Mutate(
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

	failureDomainVar, err := variables.Get[string](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("AWS failure domain variable for worker not defined")
			return nil
		}
		return err
	}

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		failureDomainVar,
	)

	// Check if this is a MachineDeployment
	if obj.GetKind() != "MachineDeployment" || obj.GetAPIVersion() != clusterv1.GroupVersion.String() {
		log.V(5).Info("not a MachineDeployment, skipping")
		return nil
	}

	log.WithValues(
		"patchedObjectKind", obj.GetKind(),
		"patchedObjectName", client.ObjectKeyFromObject(obj),
	).Info("setting failure domain in worker MachineDeployment spec")

	if err := unstructured.SetNestedField(
		obj.Object,
		failureDomainVar,
		"spec", "template", "spec", "failureDomain",
	); err != nil {
		return err
	}

	return nil
}
