// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package extraapiservercertsans

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "extraAPIServerCertSANs"
)

type extraAPIServerCertSANsPatchHandler struct {
	variableName      string
	variableFieldPath []string
	client            ctrlclient.Reader
}

func NewPatch(
	cl ctrlclient.Reader,
) *extraAPIServerCertSANsPatchHandler {
	return newExtraAPIServerCertSANsPatchHandler(clusterconfig.MetaVariableName, cl, VariableName)
}

func newExtraAPIServerCertSANsPatchHandler(
	variableName string,
	cl ctrlclient.Reader,
	variableFieldPath ...string,
) *extraAPIServerCertSANsPatchHandler {
	return &extraAPIServerCertSANsPatchHandler{
		variableName:      variableName,
		client:            cl,
		variableFieldPath: variableFieldPath,
	}
}

func (h *extraAPIServerCertSANsPatchHandler) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	clusterKey client.ObjectKey,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)
	cluster := &capiv1.Cluster{}
	if err := h.client.Get(ctx, clusterKey, cluster); err != nil {
		return err
	}
	deafultAPICertSANs := getDefaultAPIServerSANs(cluster)
	extraAPIServerCertSANsVar, found, err := variables.Get[v1alpha1.ExtraAPIServerCertSANs](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		return err
	}
	if !found && len(deafultAPICertSANs) == 0 {
		log.V(5).Info("No Extra API server cert SANs needed to be added")
		return nil
	}

	extraSans := deDup(extraAPIServerCertSANsVar, defaultDockerCertSANs)

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		extraAPIServerCertSANsVar,
	)

	return patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("adding API server extra cert SANs in kubeadm config spec")

			if obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration == nil {
				obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration = &bootstrapv1.ClusterConfiguration{}
			}
			obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.CertSANs = extraSans
			return nil
		},
	)
}

func getDefaultAPIServerSANs(cluster *capiv1.Cluster) []string {
	provider, ok := cluster.Labels[capiv1.ProviderNameLabel]
	if !ok {
		return []string{}
	}
	switch provider {
	case "docker":
		return v1alpha1.DefaultDockerCertSANs
	default:
		return []string{}
	}
}

func deDup(a, b []string) []string {
	found := map[string]bool{}
	for _, s := range a {
		if _, ok := found[s]; !ok {
			found[s] = true
		}
	}
	for _, s := range b {
		if _, ok := found[s]; !ok {
			found[s] = true
		}
	}
	ret := make([]string, 0, len(found))
	for k := range found {
		ret = append(ret, k)
	}
	return ret
}
