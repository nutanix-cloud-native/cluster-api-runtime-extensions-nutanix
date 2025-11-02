// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package konnectoragent

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	commonhandlers "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

const (
	defaultHelmReleaseName       = "konnector-agent"
	defaultHelmReleaseNamespace  = "ntnx-system"
	defaultK8sAgentName          = "konnector-agent"
	defaultCredentialsSecretName = defaultK8sAgentName

	cleanupStatusCompleted  = "completed"
	cleanupStatusInProgress = "in-progress"
	cleanupStatusNotStarted = "not-started"
	cleanupStatusTimedOut   = "timed-out"

	// helmUninstallTimeout is the maximum time to wait for HelmChartProxy deletion
	// before giving up and allowing cluster deletion to proceed.
	helmUninstallTimeout = 5 * time.Minute

	// maxClusterNameLength is the maximum cluster name length supported by Prism Central.
	maxClusterNameLength = 40
)

type Config struct {
	*options.GlobalOptions
	helmAddonConfig *addons.HelmAddonConfig
}

func NewConfig(globalOptions *options.GlobalOptions) *Config {
	return &Config{
		GlobalOptions: globalOptions,
		helmAddonConfig: addons.NewHelmAddonConfig(
			"default-konnector-agent-helm-values-template",
			defaultHelmReleaseNamespace,
			defaultHelmReleaseName,
		),
	}
}

func (c *Config) AddFlags(prefix string, flags *pflag.FlagSet) {
	c.helmAddonConfig.AddFlags(prefix+".helm-addon", flags)
}

type DefaultKonnectorAgent struct {
	client              ctrlclient.Client
	config              *Config
	helmChartInfoGetter *config.HelmChartGetter

	variableName string   // points to the global config variable
	variablePath []string // path of this variable on the global config variable
}

var (
	_ commonhandlers.Named                   = &DefaultKonnectorAgent{}
	_ lifecycle.AfterControlPlaneInitialized = &DefaultKonnectorAgent{}
	_ lifecycle.BeforeClusterUpgrade         = &DefaultKonnectorAgent{}
	_ lifecycle.BeforeClusterDelete          = &DefaultKonnectorAgent{}
)

func New(
	c ctrlclient.Client,
	cfg *Config,
	helmChartInfoGetter *config.HelmChartGetter,
) *DefaultKonnectorAgent {
	return &DefaultKonnectorAgent{
		client:              c,
		config:              cfg,
		helmChartInfoGetter: helmChartInfoGetter,
		variableName:        v1alpha1.ClusterConfigVariableName,
		variablePath:        []string{"addons", v1alpha1.KonnectorAgentVariableName},
	}
}

func (n *DefaultKonnectorAgent) Name() string {
	return "KonnectorAgentHandler"
}

func (n *DefaultKonnectorAgent) AfterControlPlaneInitialized(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	resp *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	n.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (n *DefaultKonnectorAgent) BeforeClusterUpgrade(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterUpgradeRequest,
	resp *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	commonResponse := &runtimehooksv1.CommonResponse{}
	n.apply(ctx, &req.Cluster, commonResponse)
	resp.Status = commonResponse.GetStatus()
	resp.Message = commonResponse.GetMessage()
}

func (n *DefaultKonnectorAgent) apply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	resp *runtimehooksv1.CommonResponse,
) {
	clusterKey := ctrlclient.ObjectKeyFromObject(cluster)

	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		clusterKey,
	)

	varMap := variables.ClusterVariablesToVariablesMap(cluster.Spec.Topology.Variables)
	k8sAgentVar, err := variables.Get[apivariables.NutanixKonnectorAgent](
		varMap,
		n.variableName,
		n.variablePath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.
				Info(
					"Skipping Konnector Agent handler," +
						"cluster does not specify request Konnector Agent addon deployment",
				)
			return
		}
		log.Error(
			err,
			"failed to read Konnector Agent variable from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read Konnector Agent variable from cluster definition: %v",
				err,
			),
		)
		return
	}

	// Ensure pc credentials are provided
	if k8sAgentVar.Credentials == nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage("name of the Secret containing PC credentials must be set")
		return
	}

	// It's possible to have the credentials Secret be created by the Helm chart.
	// However, that would leave the credentials visible in the HelmChartProxy.
	// Instead, we'll create the Secret on the remote cluster and reference it in the Helm values.
	err = handlersutils.EnsureClusterOwnerReferenceForObject(
		ctx,
		n.client,
		corev1.TypedLocalObjectReference{
			Kind: "Secret",
			Name: k8sAgentVar.Credentials.SecretRef.Name,
		},
		cluster,
	)
	if err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("error updating owner references on Nutanix k8s agent source Secret: %v",
				err,
			),
		)
		return
	}
	key := ctrlclient.ObjectKey{
		Name:      defaultCredentialsSecretName,
		Namespace: defaultHelmReleaseNamespace,
	}
	err = handlersutils.CopySecretToRemoteCluster(
		ctx,
		n.client,
		k8sAgentVar.Credentials.SecretRef.Name,
		key,
		cluster,
	)
	if err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("error creating Nutanix k8s agent Credentials Secret on the remote cluster: %v",
				err,
			),
		)
		return
	}

	var strategy addons.Applier
	helmChart, err := n.helmChartInfoGetter.For(ctx, log, config.KonnectorAgent)
	if err != nil {
		log.Error(
			err,
			"failed to get configmap with helm settings",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to get configuration to create helm addon: %v",
				err,
			),
		)
		return
	}
	clusterConfigVar, err := variables.Get[apivariables.ClusterConfigSpec](
		varMap,
		v1alpha1.ClusterConfigVariableName,
	)
	if err != nil {
		log.Error(
			err,
			"failed to read clusterConfig variable from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read clusterConfig variable from cluster definition: %v",
				err,
			),
		)
		return
	}
	strategy = addons.NewHelmAddonApplier(
		n.config.helmAddonConfig,
		n.client,
		helmChart,
	).WithValueTemplater(templateValuesFunc(clusterConfigVar.Nutanix, cluster))

	if err := strategy.Apply(ctx, cluster, n.config.DefaultsNamespace(), log); err != nil {
		log.Error(err, "Helm strategy Apply failed")
		err = fmt.Errorf("failed to apply Konnector Agent addon: %w", err)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

func templateValuesFunc(
	nutanixConfig *v1alpha1.NutanixSpec, cluster *clusterv1.Cluster,
) func(*clusterv1.Cluster, string) (string, error) {
	return func(_ *clusterv1.Cluster, valuesTemplate string) (string, error) {
		joinQuoted := template.FuncMap{
			"joinQuoted": func(items []string) string {
				for i, item := range items {
					items[i] = fmt.Sprintf("%q", item)
				}
				return strings.Join(items, ", ")
			},
		}
		helmValuesTemplate, err := template.New("").Funcs(joinQuoted).Parse(valuesTemplate)
		if err != nil {
			return "", fmt.Errorf("failed to parse Helm values template: %w", err)
		}

		type input struct {
			AgentName            string
			PrismCentralHost     string
			PrismCentralPort     uint16
			PrismCentralInsecure bool
			ClusterName          string
		}

		address, port, err := nutanixConfig.PrismCentralEndpoint.ParseURL()
		if err != nil {
			return "", err
		}

		// Prism Central has a limit on cluster name length
		// Truncate the cluster name if it exceeds this limit
		clusterName := cluster.Name
		if len(clusterName) > maxClusterNameLength {
			clusterName = clusterName[:maxClusterNameLength]
		}

		templateInput := input{
			AgentName:        defaultK8sAgentName,
			PrismCentralHost: address,
			PrismCentralPort: port,
			// TODO: remove this once we have a way to set this.
			// need to add support to accept PC's trust bundle in agent(it's not implemented currently)
			PrismCentralInsecure: true,
			ClusterName:          clusterName,
		}

		var b bytes.Buffer
		err = helmValuesTemplate.Execute(&b, templateInput)
		if err != nil {
			return "", fmt.Errorf("failed setting PrismCentral configuration in template: %w", err)
		}

		return b.String(), nil
	}
}

func (n *DefaultKonnectorAgent) BeforeClusterDelete(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterDeleteRequest,
	resp *runtimehooksv1.BeforeClusterDeleteResponse,
) {
	cluster := &req.Cluster
	clusterKey := ctrlclient.ObjectKeyFromObject(cluster)

	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		clusterKey,
	)

	varMap := variables.ClusterVariablesToVariablesMap(cluster.Spec.Topology.Variables)
	_, err := variables.Get[apivariables.NutanixKonnectorAgent](
		varMap,
		n.variableName,
		n.variablePath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.Info(
				"Skipping Konnector Agent cleanup, addon not specified in cluster definition",
			)
			resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
			return
		}
		log.Error(
			err,
			"failed to read Konnector Agent variable from cluster definition",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(
			fmt.Sprintf("failed to read Konnector Agent variable from cluster definition: %v",
				err,
			),
		)
		return
	}

	// Check if cleanup is already in progress or completed
	cleanupStatus, statusMsg, err := n.checkCleanupStatus(ctx, cluster, log)
	if err != nil {
		log.Error(err, "Failed to check cleanup status")
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		return
	}

	switch cleanupStatus {
	case cleanupStatusCompleted:
		log.Info("Konnector Agent cleanup already completed")
		resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
		return
	case cleanupStatusTimedOut:
		// Log the error prominently and block cluster deletion
		log.Error(
			fmt.Errorf("konnector Agent helm uninstallation timed out"),
			"ERROR: Konnector Agent cleanup timed out - blocking cluster deletion",
			"details", statusMsg,
			"action", "Manual intervention required - check HelmChartProxy status and remove finalizers if needed",
		)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf(
			"Konnector Agent helm uninstallation timed out after %v. "+
				"The HelmChartProxy is stuck in deletion state. "+
				"Manual intervention required: Check HelmChartProxy status and remove finalizers if needed. "+
				"Details: %s",
			helmUninstallTimeout,
			statusMsg,
		))
		return
	case cleanupStatusInProgress:
		log.Info("Konnector Agent cleanup in progress, requesting retry", "details", statusMsg)
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetRetryAfterSeconds(5) // Retry after 5 seconds
		resp.SetMessage(fmt.Sprintf(
			"Konnector Agent cleanup in progress. Waiting for HelmChartProxy deletion to complete. %s",
			statusMsg,
		))
		return
	case cleanupStatusNotStarted:
		log.Info("Starting Konnector Agent cleanup")
		// Proceed with cleanup below
	}

	err = n.deleteHelmChartProxy(ctx, cluster, log)
	if err != nil {
		log.Error(err, "Failed to delete HelmChartProxy")
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("Failed to delete Konnector Agent HelmChartProxy: %v", err))
		return
	}

	// After initiating cleanup, request a retry to monitor completion
	log.Info("Konnector Agent cleanup initiated, will monitor progress")
	resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
	resp.SetRetryAfterSeconds(5) // Quick retry to start monitoring
	resp.SetMessage("Konnector Agent cleanup initiated. Waiting for HelmChartProxy deletion to start.")
}

func (n *DefaultKonnectorAgent) deleteHelmChartProxy(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	clusterUUID, ok := cluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey]
	if !ok {
		return fmt.Errorf(
			"cluster UUID not found in cluster annotations - missing key %s",
			v1alpha1.ClusterUUIDAnnotationKey,
		)
	}

	// Create HelmChartProxy with the same naming pattern used during creation
	hcp := &caaphv1.HelmChartProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", defaultHelmReleaseName, clusterUUID),
			Namespace: cluster.Namespace,
		},
	}

	// First, try to gracefully trigger helm uninstall while cluster is still accessible
	log.Info("Initiating graceful deletion of Konnector Agent", "name", hcp.Name, "namespace", hcp.Namespace)

	// Get the current HCP to check if it exists and get its current state
	currentHCP := &caaphv1.HelmChartProxy{}
	err := n.client.Get(ctx, ctrlclient.ObjectKeyFromObject(hcp), currentHCP)
	if err != nil {
		if ctrlclient.IgnoreNotFound(err) == nil {
			log.Info("Konnector Agent HelmChartProxy is not present on cluster", "name", hcp.Name)
			return nil
		}
		return fmt.Errorf("failed to get HelmChartProxy %q: %w", ctrlclient.ObjectKeyFromObject(hcp), err)
	}

	// Now delete the HelmChartProxy - CAAPH will handle the helm uninstall
	log.Info("Deleting Konnector Agent HelmChartProxy", "name", hcp.Name, "namespace", hcp.Namespace)
	if err := n.client.Delete(ctx, currentHCP); err != nil {
		if ctrlclient.IgnoreNotFound(err) == nil {
			log.Info("Konnector Agent HelmChartProxy already deleted", "name", hcp.Name)
			return nil
		}
		return fmt.Errorf(
			"failed to delete Konnector Agent HelmChartProxy %q: %w",
			ctrlclient.ObjectKeyFromObject(hcp),
			err,
		)
	}

	return nil
}

// checkCleanupStatus checks the current status of Konnector Agent cleanup.
// Returns: status ("completed", "in-progress", "not-started", or "timed-out"), status message, and error.
func (n *DefaultKonnectorAgent) checkCleanupStatus(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) (status, statusMsg string, err error) {
	clusterUUID, ok := cluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey]
	if !ok {
		return cleanupStatusCompleted, "No cluster UUID found, assuming no agent installed", nil
	}

	// Check if HelmChartProxy exists
	hcp := &caaphv1.HelmChartProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", defaultHelmReleaseName, clusterUUID),
			Namespace: cluster.Namespace,
		},
	}

	err = n.client.Get(ctx, ctrlclient.ObjectKeyFromObject(hcp), hcp)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("HelmChartProxy not found, cleanup completed", "name", hcp.Name)
			return cleanupStatusCompleted, "HelmChartProxy successfully deleted", nil
		}
		return "", "", fmt.Errorf("failed to get HelmChartProxy %q: %w", ctrlclient.ObjectKeyFromObject(hcp), err)
	}

	// HCP exists - check if it's being deleted
	if hcp.DeletionTimestamp != nil {
		// Check if deletion has timed out
		deletionDuration := time.Since(hcp.DeletionTimestamp.Time)
		if deletionDuration > helmUninstallTimeout {
			statusMsg := fmt.Sprintf(
				"HelmChartProxy %q has been in deletion state for %v (timeout: %v). "+
					"Possible causes: stuck finalizers, helm uninstall failure, or workload cluster unreachable. "+
					"HelmChartProxy status: %+v",
				ctrlclient.ObjectKeyFromObject(hcp),
				deletionDuration,
				helmUninstallTimeout,
				hcp.Status,
			)
			log.Error(
				fmt.Errorf("helm uninstall timeout exceeded"),
				"HelmChartProxy deletion timed out",
				"name", hcp.Name,
				"deletionTimestamp", hcp.DeletionTimestamp.Time,
				"duration", deletionDuration,
				"timeout", helmUninstallTimeout,
				"finalizers", hcp.Finalizers,
				"status", hcp.Status,
			)
			return cleanupStatusTimedOut, statusMsg, nil
		}

		statusMsg := fmt.Sprintf(
			"HelmChartProxy is being deleted (in progress for %v, timeout in %v)",
			deletionDuration,
			helmUninstallTimeout-deletionDuration,
		)
		log.Info("HelmChartProxy is being deleted, cleanup in progress",
			"name", hcp.Name,
			"deletionDuration", deletionDuration,
			"remainingTime", helmUninstallTimeout-deletionDuration,
		)
		return cleanupStatusInProgress, statusMsg, nil
	}

	// HCP exists and is not being deleted
	log.Info("HelmChartProxy exists, cleanup not started", "name", hcp.Name)
	return cleanupStatusNotStarted, "HelmChartProxy exists and needs to be deleted", nil
}
