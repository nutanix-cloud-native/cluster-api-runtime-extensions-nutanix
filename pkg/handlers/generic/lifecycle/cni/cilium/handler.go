// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cilium

import (
	"context"
	"fmt"

	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	commonhandlers "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	caaphv1 "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/k8s/client"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
)

const (
	defaultHelmRepositoryURL    = "https://helm.cilium.io/"
	defaultCiliumVersion        = "1.15.0-pre.3"
	defaultChartName            = "cilium"
	defaultHelmReleaseName      = "cilium"
	defaultHelmReleaseNamespace = "kube-system"
)

type CNIConfig struct {
	defaultsNamespace                                        string
	defaultProviderInstallationValuesTemplatesConfigMapNames map[string]string
}

func (c *CNIConfig) AddFlags(prefix string, flags *pflag.FlagSet) {
	flags.StringVar(
		&c.defaultsNamespace,
		prefix+".defaultsNamespace",
		corev1.NamespaceDefault,
		"namespace of the default values ConfigMaps",
	)

	flags.StringToStringVar(
		&c.defaultProviderInstallationValuesTemplatesConfigMapNames,
		prefix+".default-values-templates-configmap-names",
		map[string]string{
			"DockerCluster": "cilium-cni-values-template-dockercluster",
		},
		"map of provider cluster implementation type to default values ConfigMap name",
	)
}

type CiliumCNI struct {
	client ctrlclient.Client
	config *CNIConfig

	variableName string
	variablePath []string
}

var (
	_ commonhandlers.Named                   = &CiliumCNI{}
	_ lifecycle.AfterControlPlaneInitialized = &CiliumCNI{}
)

func New(
	c ctrlclient.Client,
	cfg *CNIConfig,
) *CiliumCNI {
	return &CiliumCNI{
		client:       c,
		config:       cfg,
		variableName: clusterconfig.MetaVariableName,
		variablePath: []string{"addons", v1alpha1.CNIVariableName},
	}
}

func (s *CiliumCNI) Name() string {
	return "CiliumCNI"
}

func (s *CiliumCNI) AfterControlPlaneInitialized(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	resp *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	clusterKey := ctrlclient.ObjectKeyFromObject(&req.Cluster)

	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		clusterKey,
	)

	varMap := variables.ClusterVariablesToVariablesMap(req.Cluster.Spec.Topology.Variables)

	cniVar, found, err := variables.Get[v1alpha1.CNI](varMap, s.variableName, s.variablePath...)
	if err != nil {
		log.Error(
			err,
			"failed to read CNI provider from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read CNI provider from cluster definition: %v",
				err,
			),
		)
		return
	}
	if !found || cniVar.Provider != v1alpha1.CNIProviderCilium {
		log.V(4).Info(
			fmt.Sprintf(
				"Skipping Cilium CNI handler, cluster does not specify %q as value of CNI provider variable",
				v1alpha1.CNIProviderCilium,
			),
		)
		return
	}

	infraKind := req.Cluster.Spec.InfrastructureRef.Kind
	defaultInstallationConfigMapName, ok := s.config.defaultProviderInstallationValuesTemplatesConfigMapNames[infraKind]
	if !ok {
		log.V(4).Info(
			fmt.Sprintf(
				"Skipping Cilium CNI handler, no default installation values ConfigMap configured for infrastructure provider %q",
				req.Cluster.Spec.InfrastructureRef.Kind,
			),
		)
		return
	}

	log.Info("Retrieving Cilium installation values template for cluster")
	valuesTemplateConfigMap, err := s.retrieveValuesTemplateConfigMap(
		ctx,
		defaultInstallationConfigMapName,
	)
	if err != nil {
		log.Error(
			err,
			"failed to retrieve Cilium installation values template ConfigMap for cluster",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf(
				"failed to retrieve Cilium installation values template ConfigMap for cluster: %v",
				err,
			),
		)
		return
	}

	hcp := &caaphv1.HelmChartProxy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: caaphv1.GroupVersion.String(),
			Kind:       "HelmChartProxy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Cluster.Namespace,
			Name:      "cilium-cni-installation-" + req.Cluster.Name,
		},
		Spec: caaphv1.HelmChartProxySpec{
			RepoURL:   defaultHelmRepositoryURL,
			ChartName: defaultChartName,
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{capiv1.ClusterNameLabel: req.Cluster.Name},
			},
			ReleaseNamespace: defaultHelmReleaseNamespace,
			ReleaseName:      defaultHelmReleaseName,
			Version:          defaultCiliumVersion,
			ValuesTemplate:   valuesTemplateConfigMap.Data["values.yaml"],
		},
	}

	if err := client.ServerSideApply(ctx, s.client, hcp); err != nil {
		log.Error(
			err,
			"failed to apply Cilium CNI installation HelmChartProxy",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf(
				"failed to apply Cilium CNI installation HelmChartProxy: %v",
				err,
			),
		)
		return
	}

	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

func (s *CiliumCNI) retrieveValuesTemplateConfigMap(
	ctx context.Context,
	configMapName string,
) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: s.config.defaultsNamespace,
			Name:      configMapName,
		},
	}
	configMapObjName := ctrlclient.ObjectKeyFromObject(
		configMap,
	)
	err := s.client.Get(ctx, configMapObjName, configMap)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to retrieve installation values template ConfigMap %q: %w",
			configMapObjName,
			err,
		)
	}

	return configMap, nil
}
