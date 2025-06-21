// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package kubeproxymode

import (
	"context"
	"fmt"
	"slices"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "kubeProxy"

	kubeProxyConfigYAMLTemplate = `
---
apiVersion: kubeproxy.config.k8s.io/v1alpha1
kind: KubeProxyConfiguration
mode: %s
`

	// kubeProxyConfigYAMLTemplateForDockerProvider is the kube-proxy configuration template for Docker provider.
	// CAPD already configures some stuff in KubeProxyConfiguration, so we only need to set the mode.
	kubeProxyConfigYAMLTemplateForDockerProvider = `
mode: %s
`
)

type kubeProxyMode struct {
	variableName      string
	variableFieldPath []string
}

func NewPatch() *kubeProxyMode {
	return newKubeProxyModePatch(
		v1alpha1.ClusterConfigVariableName,
		VariableName,
		"mode",
	)
}

func newKubeProxyModePatch(
	variableName string,
	variableFieldPath ...string,
) *kubeProxyMode {
	return &kubeProxyMode{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *kubeProxyMode) Mutate(
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

	cluster, err := clusterGetter(ctx)
	if err != nil {
		log.Error(err, "failed to get cluster for kube proxy mode mutation")
		return fmt.Errorf("failed to get cluster for kube proxy mode mutation: %w", err)
	}

	isSkipProxy := false
	if cluster.Spec.Topology != nil {
		_, isSkipProxy = cluster.Spec.Topology.ControlPlane.Metadata.Annotations[controlplanev1.SkipKubeProxyAnnotation]
	}

	kubeProxyMode, err := variables.Get[v1alpha1.KubeProxyMode](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil && !variables.IsNotFoundError(err) {
		return err
	}

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		kubeProxyMode,
	)

	if kubeProxyMode == "" && !isSkipProxy {
		log.V(5).Info("kube proxy mode is not set or skipped, skipping mutation")
		return nil
	}

	return patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.ControlPlane(),
		log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("adding kube proxy mode to control plane kubeadm config spec")

			if isSkipProxy {
				log.Info(
					"cluster controlplane contains controlplane.cluster.x-k8s.io/skip-kube-proxy annotation, " +
						"skipping kube-proxy addon",
				)
				if obj.Spec.Template.Spec.KubeadmConfigSpec.InitConfiguration == nil {
					obj.Spec.Template.Spec.KubeadmConfigSpec.InitConfiguration = &bootstrapv1.InitConfiguration{}
				}
				initConfiguration := obj.Spec.Template.Spec.KubeadmConfigSpec.InitConfiguration
				if !slices.Contains(initConfiguration.SkipPhases, "addon/kube-proxy") {
					initConfiguration.SkipPhases = append(
						initConfiguration.SkipPhases,
						"addon/kube-proxy",
					)
				}

				return nil
			}

			switch kubeProxyMode {
			case v1alpha1.KubeProxyModeIPTables, v1alpha1.KubeProxyModeNFTables:
				kubeProxyConfigProviderTemplate := templateForClusterProvider(cluster)

				kubeProxyConfig := bootstrapv1.File{
					Path:        "/etc/kubernetes/kubeproxy-config.yaml",
					Owner:       "root:root",
					Permissions: "0644",
					Content:     fmt.Sprintf(kubeProxyConfigProviderTemplate, kubeProxyMode),
				}
				obj.Spec.Template.Spec.KubeadmConfigSpec.Files = append(
					obj.Spec.Template.Spec.KubeadmConfigSpec.Files,
					kubeProxyConfig,
				)
				mergeKubeProxyConfigCmd := "/bin/sh -ec 'cat /etc/kubernetes/kubeproxy-config.yaml >> /run/kubeadm/kubeadm.yaml'"
				obj.Spec.Template.Spec.KubeadmConfigSpec.PreKubeadmCommands = append(
					obj.Spec.Template.Spec.KubeadmConfigSpec.PreKubeadmCommands,
					mergeKubeProxyConfigCmd,
				)
			default:
				return fmt.Errorf("unknown kube proxy mode %q", kubeProxyMode)
			}

			return nil
		},
	)
}

// templateForClusterProvider returns the kube-proxy config template based on the cluster provider.
func templateForClusterProvider(cluster *clusterv1.Cluster) string {
	switch utils.GetProvider(cluster) {
	case "docker":
		return kubeProxyConfigYAMLTemplateForDockerProvider
	default:
		return kubeProxyConfigYAMLTemplate
	}
}
