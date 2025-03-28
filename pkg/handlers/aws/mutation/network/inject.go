// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capav1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "network"
)

type awsNetworkPatchHandler struct {
	variableName      string
	variableFieldPath []string
}

func NewPatch() *awsNetworkPatchHandler {
	return newAWSPatchPatchHandler(
		v1alpha1.ClusterConfigVariableName,
		v1alpha1.AWSVariableName,
		VariableName,
	)
}

func newAWSPatchPatchHandler(
	variableName string,
	variableFieldPath ...string,
) *awsNetworkPatchHandler {
	return &awsNetworkPatchHandler{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *awsNetworkPatchHandler) Mutate(
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

	networkVar, err := variables.Get[v1alpha1.AWSNetwork](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("AWS Network variable not defined")
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
		networkVar,
	)

	return patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.InfrastructureCluster(capav1.GroupVersion.Version, "AWSClusterTemplate"),
		log,
		func(obj *capav1.AWSClusterTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("setting Network in AWSCluster spec")

			if networkVar.VPC != nil &&
				networkVar.VPC.ID != "" {
				obj.Spec.Template.Spec.NetworkSpec.VPC = capav1.VPCSpec{
					ID: networkVar.VPC.ID,
				}
			}

			if len(networkVar.Subnets) > 0 {
				subnets := make([]capav1.SubnetSpec, 0)
				for _, subnet := range networkVar.Subnets {
					if subnet.ID == "" {
						continue
					}
					subnets = append(subnets, capav1.SubnetSpec{
						ID: subnet.ID,
					})
				}
				obj.Spec.Template.Spec.NetworkSpec.Subnets = subnets
			}

			return nil
		},
	)
}
