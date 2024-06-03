// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controlplanevirtualip

import (
	"context"
	"fmt"

	"github.com/spf13/pflag"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/controlplanevirtualip/providers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "controlPlaneEndpoint"
)

type Config struct {
	*options.GlobalOptions

	defaultKubeVIPConfigMapName string
}

func (c *Config) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultKubeVIPConfigMapName,
		prefix+".default-kube-vip-template-configmap-name",
		"default-kube-vip-template",
		"default ConfigMap name that holds the kube-vip template used for the control-plane virtual IP",
	)
}

type ControlPlaneVirtualIP struct {
	client client.Reader
	config *Config

	variableName      string
	variableFieldPath []string
}

// NewControlPlaneVirtualIP is different from other generic handlers.
// It requires variableName and variableFieldPath to be passed from another provider specific handler.
// The code is here to be shared across different providers.
func NewControlPlaneVirtualIP(
	cl client.Reader,
	config *Config,
	variableName string,
	variableFieldPath ...string,
) *ControlPlaneVirtualIP {
	return &ControlPlaneVirtualIP{
		client:            cl,
		config:            config,
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *ControlPlaneVirtualIP) Mutate(
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

	controlPlaneEndpointVar, err := variables.Get[v1alpha1.ControlPlaneEndpointSpec](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("ControlPlaneEndpoint variable not defined")
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
		controlPlaneEndpointVar,
	)

	if controlPlaneEndpointVar.VirtualIPSpec == nil {
		log.V(5).Info("ControlPlane VirtualIP not set")
		return nil
	}

	cluster, err := clusterGetter(ctx)
	if err != nil {
		log.Error(
			err,
			"failed to get cluster from ControlPlaneVirtualIP mutation handler",
		)
		return err
	}

	var virtualIPProvider providers.Provider
	// only kube-vip is supported, but more providers can be added in the future
	if controlPlaneEndpointVar.VirtualIPSpec.Provider == v1alpha1.VirtualIPProviderKubeVIP {
		virtualIPProvider = providers.NewKubeVIPFromConfigMapProvider(
			h.client,
			h.config.defaultKubeVIPConfigMapName,
			h.config.DefaultsNamespace(),
		)
	}

	return patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.ControlPlane(),
		log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			virtualIPProviderFiles, preKubeadmCommands, postKubeadmCommands, generateErr :=
				virtualIPProvider.GenerateFilesAndCommands(
					ctx,
					controlPlaneEndpointVar,
					cluster,
				)
			if generateErr != nil {
				return generateErr
			}

			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info(fmt.Sprintf(
				"adding %s static Pod file to control plane kubeadm config spec",
				virtualIPProvider.Name(),
			))
			obj.Spec.Template.Spec.KubeadmConfigSpec.Files = append(
				obj.Spec.Template.Spec.KubeadmConfigSpec.Files,
				virtualIPProviderFiles...,
			)

			if len(preKubeadmCommands) > 0 {
				log.WithValues(
					"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
					"patchedObjectName", client.ObjectKeyFromObject(obj),
				).Info(fmt.Sprintf(
					"adding %s preKubeadmCommands to control plane kubeadm config spec",
					virtualIPProvider.Name(),
				))
				obj.Spec.Template.Spec.KubeadmConfigSpec.PreKubeadmCommands = append(
					obj.Spec.Template.Spec.KubeadmConfigSpec.PreKubeadmCommands,
					preKubeadmCommands...,
				)
			}

			if len(postKubeadmCommands) > 0 {
				log.WithValues(
					"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
					"patchedObjectName", client.ObjectKeyFromObject(obj),
				).Info(fmt.Sprintf(
					"adding %s postKubeadmCommands to control plane kubeadm config spec",
					virtualIPProvider.Name(),
				))
				obj.Spec.Template.Spec.KubeadmConfigSpec.PostKubeadmCommands = append(
					obj.Spec.Template.Spec.KubeadmConfigSpec.PostKubeadmCommands,
					postKubeadmCommands...,
				)
			}

			return nil
		},
	)
}
