// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controlplaneloadbalancer

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
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "controlPlaneLoadBalancer"
)

type awsControlPlaneLoadBalancer struct {
	variableName      string
	variableFieldPath []string
}

func NewPatch() *awsControlPlaneLoadBalancer {
	return newAWSControlPlaneLoadBalancer(
		clusterconfig.MetaVariableName,
		v1alpha1.AWSVariableName,
		VariableName,
	)
}

func newAWSControlPlaneLoadBalancer(
	variableName string,
	variableFieldPath ...string,
) *awsControlPlaneLoadBalancer {
	return &awsControlPlaneLoadBalancer{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *awsControlPlaneLoadBalancer) Mutate(
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

	controlPlaneLoadBalancerVar, err := variables.Get[v1alpha1.AWSLoadBalancerSpec](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("AWS ControlPlaneLoadBalancer variable not defined")
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
		controlPlaneLoadBalancerVar,
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
			).Info("setting ControlPlaneLoadBalancer in AWSCluster spec")

			controlPlaneLoadBalancer := obj.Spec.Template.Spec.ControlPlaneLoadBalancer
			if controlPlaneLoadBalancer == nil {
				obj.Spec.Template.Spec.ControlPlaneLoadBalancer = &capav1.AWSLoadBalancerSpec{}
			}
			obj.Spec.Template.Spec.ControlPlaneLoadBalancer.Scheme = controlPlaneLoadBalancerVar.Scheme

			return nil
		},
	)
}
