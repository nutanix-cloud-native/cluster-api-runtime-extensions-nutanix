// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package kubeproxymode

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"slices"
	"text/template"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	eksv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/controlplane/eks/api/v1beta2"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "kubeProxy"

	// addKubeProxyModeToExistingKubeProxyConfiguration is a sed command to add the kube-proxy mode to
	// an existing KubeProxyConfiguration present in the kubeadm config file. If there is no existing
	// KubeProxyConfiguration, it will exit with a non-zero status code which allows to run the fallback
	// command to append the KubeProxyConfiguration specified in the template above to the kubeadm config file.
	addKubeProxyModeToExistingKubeProxyConfiguration = `grep -q "^kind: KubeProxyConfiguration$" %[1]s && sed -i -e "s/^\(kind: KubeProxyConfiguration\)$/\1\nmode: %[2]s/" %[1]s` //nolint:lll // Just a long command.

	kubeadmConfigFilePath = "/run/kubeadm/kubeadm.yaml"
)

var (
	//go:embed embedded/kubeproxyconfig.yaml
	kubeProxyConfigYAML []byte

	kubeProxyConfigTemplate = template.Must(template.New("kubeProxyConfig").Parse(string(kubeProxyConfigYAML)))
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

	if kubeProxyMode == "" {
		log.V(5).Info("kube proxy mode is not set, skipping mutation")
		return nil
	}

	if err := patches.MutateIfApplicable(
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

			switch kubeProxyMode {
			case v1alpha1.KubeProxyModeDisabled:
				log.Info("disabling kube-proxy addon")

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
			case v1alpha1.KubeProxyModeIPTables, v1alpha1.KubeProxyModeNFTables:
				return addKubeProxyConfigFileAndCommand(obj, kubeProxyMode)
			default:
				return fmt.Errorf("unknown kube proxy mode %q", kubeProxyMode)
			}
		},
	); err != nil {
		return err
	}

	if err := patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		clusterv1.PatchSelector{
			APIVersion: eksv1.GroupVersion.String(),
			Kind:       "AWSManagedControlPlaneTemplate",
			MatchResources: clusterv1.PatchSelectorMatch{
				ControlPlane: true,
			},
		},
		log,
		func(obj *eksv1.AWSManagedControlPlaneTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("adding kube proxy mode to AWSManagedControlPlaneTemplate spec")

			if kubeProxyMode == v1alpha1.KubeProxyModeDisabled {
				log.Info("disabling kube-proxy addon")

				obj.Spec.Template.Spec.KubeProxy.Disable = true
			}

			return nil
		},
	); err != nil {
		return err
	}

	return nil
}

// addKubeProxyConfigFileAndCommand adds the kube-proxy configuration file and command to the KCPTemplate.
// It creates a KubeProxyConfiguration file with the specified mode and appends it to the kubeadm config file.
// It also adds a command to the PreKubeadmCommands to merge the kube-proxy configuration into the kubeadm config file.
// If the kubeadm config file already contains a KubeProxyConfiguration, it uses a sed command to add the mode to
// the existing configuration.
// If the kubeadm config file does not contain a KubeProxyConfiguration, it appends the new configuration
// to the kubeadm config file using a cat command.
//
// TODO: KubeProxyConfiguration should be exposed upstream in CAPI to be able to configure kube-proxy mode directly
// without the need for the messy commands in this implementation.
func addKubeProxyConfigFileAndCommand(
	obj *controlplanev1.KubeadmControlPlaneTemplate, kubeProxyMode v1alpha1.KubeProxyMode,
) error {
	templateInput := struct {
		Mode string
	}{
		Mode: string(kubeProxyMode),
	}
	var b bytes.Buffer
	err := kubeProxyConfigTemplate.Execute(&b, templateInput)
	if err != nil {
		return fmt.Errorf("failed executing kube-proxy config template: %w", err)
	}

	kubeProxyConfig := bootstrapv1.File{
		Path:        "/etc/kubernetes/kubeproxy-config.yaml",
		Owner:       "root:root",
		Permissions: "0644",
		Content:     b.String(),
	}
	obj.Spec.Template.Spec.KubeadmConfigSpec.Files = append(
		obj.Spec.Template.Spec.KubeadmConfigSpec.Files,
		kubeProxyConfig,
	)

	sedCommand := fmt.Sprintf(
		addKubeProxyModeToExistingKubeProxyConfiguration,
		kubeadmConfigFilePath,
		kubeProxyMode,
	)
	catCommand := fmt.Sprintf(
		"cat /etc/kubernetes/kubeproxy-config.yaml >>%s",
		kubeadmConfigFilePath,
	)
	mergeKubeProxyConfigCmd := fmt.Sprintf(
		"/bin/sh -ec '(%s) || (%s)'",
		sedCommand,
		catCommand,
	)

	obj.Spec.Template.Spec.KubeadmConfigSpec.PreKubeadmCommands = append(
		obj.Spec.Template.Spec.KubeadmConfigSpec.PreKubeadmCommands,
		mergeKubeProxyConfigCmd,
	)
	return nil
}
