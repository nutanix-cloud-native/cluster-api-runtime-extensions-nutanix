// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package externalcloudprovider

import (
	"context"
	"fmt"

	"github.com/blang/semver/v4"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

var versionGreaterOrEqualTo133Range = semver.MustParseRange(">=1.33.0-0")

type externalCloudProviderPatchHandler struct{}

func NewControlPlanePatch() *externalCloudProviderPatchHandler {
	return &externalCloudProviderPatchHandler{}
}

func (h *externalCloudProviderPatchHandler) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ ctrlclient.ObjectKey,
	_ mutation.ClusterGetter,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	cpVersion, err := variables.Get[string](vars, runtimehooksv1.BuiltinsName, "controlPlane", "version")
	if err != nil {
		// This builtin variable is guaranteed to be provided for control plane component patch requests so if it is not
		// found then we can safely skip this patch for this request item.
		if variables.IsFieldNotFoundError(err) {
			log.V(5).
				WithValues("variables", vars).
				Info(
					"skipping external cloud-provider flag to control plane because CP Kubernetes version is not found",
				)
			return nil
		}

		// This is a fatal error, we can't proceed without the control plane version.
		log.WithValues("variables", vars).
			Error(err, "failed to get control plane Kubernetes version from builtin variable")
		return fmt.Errorf("failed to get control plane Kubernetes version from builtin variable: %w", err)
	}

	kubernetesVersion, err := semver.ParseTolerant(cpVersion)
	if err != nil {
		log.WithValues(
			"kubernetesVersion",
			cpVersion,
		).Error(err, "failed to parse control plane Kubernetes version")
		return fmt.Errorf("failed to parse control plane Kubernetes version: %w", err)
	}

	if versionGreaterOrEqualTo133Range(kubernetesVersion) {
		log.V(5).Info(
			"skipping external cloud-provider flag to control plane kubeadm config template because Kubernetes >= 1.33.0",
		)
		return nil
	}

	if err := patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			if obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration == nil {
				obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration = &bootstrapv1.ClusterConfiguration{}
			}
			if obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.ExtraArgs == nil {
				obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.ExtraArgs = make(map[string]string, 1)
			}
			if _, ok := obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.ExtraArgs["cloud-provider"]; !ok { //nolint:lll // Easier to read this way.
				obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.ExtraArgs["cloud-provider"] = "external"
			}

			return nil
		}); err != nil {
		return err
	}

	return nil
}
