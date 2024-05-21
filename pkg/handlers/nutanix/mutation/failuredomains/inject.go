// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package failuredomains

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "failureDomains"
)

type nutanixFailureDomains struct {
	variableName      string
	variableFieldPath []string
}

func NewPatch() *nutanixFailureDomains {
	return newnutanixFailureDomains(
		v1alpha1.ClusterConfigVariableName,
		v1alpha1.NutanixVariableName,
		VariableName,
	)
}

func newnutanixFailureDomains(
	variableName string,
	variableFieldPath ...string,
) *nutanixFailureDomains {
	return &nutanixFailureDomains{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *nutanixFailureDomains) Mutate(
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

	failureDomainsVar, err := variables.Get[v1alpha1.NutanixFailureDomains](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("Nutanix failureDomains variable not defined")
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
		failureDomainsVar,
	)

	failureDomains := make([]capxv1.NutanixFailureDomain, 0, len(failureDomainsVar))
	for _, fd := range failureDomainsVar {
		subnets := make([]capxv1.NutanixResourceIdentifier, 0, len(fd.Subnets))
		for _, subnet := range fd.Subnets {
			subnets = append(subnets, capxv1.NutanixResourceIdentifier{
				Type: subnet.Type,
				Name: subnet.Name,
				UUID: subnet.UUID,
			})
		}
		cluster := capxv1.NutanixResourceIdentifier{
			Type: fd.Cluster.Type,
			Name: fd.Cluster.Name,
			UUID: fd.Cluster.UUID,
		}

		failureDomains = append(failureDomains, capxv1.NutanixFailureDomain{
			Name:         fd.Name,
			Cluster:      cluster,
			Subnets:      subnets,
			ControlPlane: fd.ControlPlane,
		})
	}

	return patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.InfrastructureCluster(capxv1.GroupVersion.Version, "NutanixClusterTemplate"),
		log,
		func(obj *capxv1.NutanixClusterTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("setting controlPlaneEndpoint in NutanixCluster spec")

			obj.Spec.Template.Spec.FailureDomains = failureDomains

			return nil
		},
	)
}
