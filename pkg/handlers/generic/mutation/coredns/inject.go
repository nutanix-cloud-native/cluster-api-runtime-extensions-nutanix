// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package coredns

import (
	"context"
	"errors"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	corednsversions "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/versions"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "coreDNS"
)

var ErrDefaultCoreDNSVersionNotFound = errors.New(
	"could not determine default CoreDNS version based on the Kubernetes version",
)

type coreDNSPatchHandler struct {
	variableName      string
	variableFieldPath []string
}

func NewPatch() *coreDNSPatchHandler {
	return newKubernetesDNSPatchHandlerPatchHandler(
		v1alpha1.ClusterConfigVariableName, v1alpha1.DNSVariableName, VariableName,
	)
}

func newKubernetesDNSPatchHandlerPatchHandler(
	variableName string,
	variableFieldPath ...string,
) *coreDNSPatchHandler {
	return &coreDNSPatchHandler{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *coreDNSPatchHandler) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ ctrlclient.ObjectKey,
	clusterGetter mutation.ClusterGetter,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	coreDNSVar, err := variables.Get[v1alpha1.CoreDNS](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if !variables.IsNotFoundError(err) {
			return err
		}
		log.V(5).Info("coreDNSVar variable not defined")
	}

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		coreDNSVar,
	)

	cluster, err := clusterGetter(ctx)
	if err != nil {
		log.Error(
			err,
			"failed to get cluster from CoreDNS mutation handler",
		)
		return err
	}

	return patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", ctrlclient.ObjectKeyFromObject(obj),
			).Info("setting CoreDNS version")

			if obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration == nil {
				obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration = &bootstrapv1.ClusterConfiguration{}
			}

			dns := obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.DNS

			// Set the CoreDNS image from the variable if it is defined.
			setFromVar(coreDNSVar.Image, &dns)

			// Always set the default if the CoreDNS image version is not defined in the variable.
			if useDefaultVersion(coreDNSVar) {
				defaultCoreDNSVersion, found := corednsversions.GetCoreDNSVersion(
					cluster.Spec.Topology.Version,
				)
				if !found {
					return ErrDefaultCoreDNSVersionNotFound
				}
				dns.ImageTag = defaultCoreDNSVersion
			}

			obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.DNS = dns

			return nil
		})
}

// setFromVar sets the CoreDNS image tag and repository from the variable if it is defined.
// If the variable is not defined, the function just returns.
func setFromVar(image *v1alpha1.Image, dns *bootstrapv1.DNS) {
	if image == nil {
		return
	}
	if image.Tag != "" {
		dns.ImageTag = image.Tag
	}
	if image.Repository != "" {
		dns.ImageRepository = image.Repository
	}
}

// useDefaultVersion returns true if the CoreDNS version should be set to the default version.
func useDefaultVersion(coreDNSVar v1alpha1.CoreDNS) bool {
	return coreDNSVar.Image == nil || coreDNSVar.Image.Tag == ""
}
