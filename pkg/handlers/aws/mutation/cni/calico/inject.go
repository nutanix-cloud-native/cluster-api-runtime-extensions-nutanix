// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package calico

import (
	"context"
	"slices"

	"github.com/go-logr/logr"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capav1 "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

const (
	// HandlerNamePatch is the name of the inject handler.
	HandlerNamePatch = "CalicoCNIPatch"
)

type calicoPatchHandler struct {
	variableName      string
	variableFieldPath []string
}

func NewPatch() *calicoPatchHandler {
	return newCalicoPatchHandler(
		v1alpha1.ClusterConfigVariableName,
		"addons",
		v1alpha1.CNIVariableName,
	)
}

func newCalicoPatchHandler(
	variableName string,
	variableFieldPath ...string,
) *calicoPatchHandler {
	return &calicoPatchHandler{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *calicoPatchHandler) Mutate(
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

	cniVar, err := variables.Get[v1alpha1.CNI](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("cni variable not defined")
			return nil
		}
		return err
	}
	if cniVar.Provider != v1alpha1.CNIProviderCalico {
		log.V(5).
			WithValues("cniProvider", cniVar.Provider).
			Info("CNI provider not defined as Calico - skipping")
		return nil
	}

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		cniVar,
	)

	return patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.InfrastructureCluster(capav1.GroupVersion.Version, "AWSClusterTemplate"),
		log,
		mutateAWSClusterTemplateFunc(log),
	)
}

func mutateAWSClusterTemplateFunc(log logr.Logger) func(obj *capav1.AWSClusterTemplate) error {
	return func(obj *capav1.AWSClusterTemplate) error {
		log.WithValues(
			"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
			"patchedObjectName", client.ObjectKeyFromObject(obj),
		).Info("setting CNI ingress rules in AWS cluster spec")

		if obj.Spec.Template.Spec.NetworkSpec.CNI == nil {
			obj.Spec.Template.Spec.NetworkSpec.CNI = &capav1.CNISpec{}
		}
		obj.Spec.Template.Spec.NetworkSpec.CNI.CNIIngressRules = addOrUpdateCNIIngressRules(
			obj.Spec.Template.Spec.NetworkSpec.CNI.CNIIngressRules,

			capav1.CNIIngressRule{
				Description: "typha (calico)",
				Protocol:    capav1.SecurityGroupProtocolTCP,
				FromPort:    5473,
				ToPort:      5473,
			},
			capav1.CNIIngressRule{
				Description: "bgp (calico)",
				Protocol:    capav1.SecurityGroupProtocolTCP,
				FromPort:    179,
				ToPort:      179,
			},
			capav1.CNIIngressRule{
				Description: "IP-in-IP (calico)",
				Protocol:    capav1.SecurityGroupProtocolIPinIP,
				FromPort:    -1,
				ToPort:      65535,
			},
			capav1.CNIIngressRule{
				Description: "node metrics (calico)",
				Protocol:    capav1.SecurityGroupProtocolTCP,
				FromPort:    9091,
				ToPort:      9091,
			},
			capav1.CNIIngressRule{
				Description: "typha metrics (calico)",
				Protocol:    capav1.SecurityGroupProtocolTCP,
				FromPort:    9093,
				ToPort:      9093,
			},
		)

		return nil
	}
}

func addOrUpdateCNIIngressRules(
	rules []capav1.CNIIngressRule, newRules ...capav1.CNIIngressRule,
) []capav1.CNIIngressRule {
	clonedRules := slices.Clone(rules)

	for _, newRule := range newRules {
		if !slices.Contains(clonedRules, newRule) {
			clonedRules = append(clonedRules, newRule)
		}
	}

	return clonedRules
}
