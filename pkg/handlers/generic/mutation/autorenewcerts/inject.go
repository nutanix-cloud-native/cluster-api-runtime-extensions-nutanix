// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package autorenewcerts

import (
	"context"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "autoRenewCertificates"
)

type autoRenewCerts struct {
	variableName      string
	variableFieldPath []string
}

func NewPatch() *autoRenewCerts {
	return newAutoRenewCerts(
		v1alpha1.ClusterConfigVariableName,
		v1alpha1.ControlPlaneConfigVariableName,
		VariableName,
	)
}

func newAutoRenewCerts(
	variableName string,
	variableFieldPath ...string,
) *autoRenewCerts {
	return &autoRenewCerts{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *autoRenewCerts) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ client.ObjectKey,
	clusterGetter mutation.ClusterGetter,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	autoRenewCertsVar, err := variables.Get[v1alpha1.AutoRenewCertificatesSpec](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("Control Plane auto renew certs variable not defined")
		} else {
			return err
		}
	}

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		autoRenewCertsVar,
	)

	return patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.ControlPlane(),
		log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			log = log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			)

			if autoRenewCertsVar.DaysBeforeExpiry == 0 {
				log.Info("removing auto renew certs config from control plane kubeadm config spec")
				obj.Spec.Template.Spec.RolloutBefore = nil
				return nil
			}

			log.Info(fmt.Sprintf(
				"adding auto renew certs config for %d days before expiry to control plane kubeadm config spec",
				autoRenewCertsVar.DaysBeforeExpiry,
			))
			if obj.Spec.Template.Spec.RolloutBefore == nil {
				obj.Spec.Template.Spec.RolloutBefore = &controlplanev1.RolloutBefore{}
			}
			obj.Spec.Template.Spec.RolloutBefore.CertificatesExpiryDays = ptr.To(
				autoRenewCertsVar.DaysBeforeExpiry,
			)

			return nil
		},
	)
}
