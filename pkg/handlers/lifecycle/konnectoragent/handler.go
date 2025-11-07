// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package konnectoragent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/template"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	prismcredentials "github.com/nutanix-cloud-native/prism-go-client/environment/credentials"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	commonhandlers "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
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

	// legacyHelmChartName is the chart name of the old helm release
	// that needs to be deleted during upgrades.
	legacyHelmChartName = "nutanix-k8s-agent1"

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
	clusterKey := ctrlclient.ObjectKeyFromObject(&req.Cluster)
	log := ctrl.LoggerFrom(ctx).WithValues("cluster", clusterKey)

	// Check if konnectorAgent is enabled before performing any operations
	varMap := variables.ClusterVariablesToVariablesMap(req.Cluster.Spec.Topology.Variables)
	_, err := variables.Get[apivariables.NutanixKonnectorAgent](
		varMap,
		n.variableName,
		n.variablePath...)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.Info("Konnector Agent addon not enabled, skipping all upgrade operations")
			resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
			return
		}
		log.Error(err, "Failed to read Konnector Agent variable")
		resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
		resp.SetMessage(fmt.Sprintf("failed to read Konnector Agent variable: %v", err))
		return
	}

	// Delete legacy helm release "nutanix-k8s-agent" if it exists
	_ = n.deleteLegacyHelmRelease(ctx, &req.Cluster, log)

	// Check if konnectorAgent is enabled and ensure secret exists
	// During upgrade, if konnectorAgent is enabled but secret doesn't exist,
	// create it from PC global secrets
	if err := n.ensureKonnectorAgentSecretFromPCGlobalSecret(ctx, &req.Cluster, log); err != nil {
		log.Error(err, "Failed to ensure konnectorAgent secret from PC global secret")
		resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
		resp.SetMessage(fmt.Sprintf("failed to ensure konnectorAgent secret from PC global secret: %v", err))
		return
	}

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

// deleteLegacyHelmRelease deletes the legacy helm release with chart name "nutanix-k8s-agent"
// from the remote cluster. This is called during cluster upgrades to clean up old releases
// before applying the new HelmChartProxy.
func (n *DefaultKonnectorAgent) deleteLegacyHelmRelease(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	clusterKey := ctrlclient.ObjectKeyFromObject(cluster)
	remoteClient, err := remote.NewClusterClient(ctx, "", n.client, clusterKey)
	if err != nil {
		return fmt.Errorf("error creating client for remote cluster: %w", err)
	}

	// List all helm release secrets in the namespace
	// Helm v3 stores releases as secrets with label "owner=helm"
	secretList := &corev1.SecretList{}
	err = remoteClient.List(
		ctx,
		secretList,
		ctrlclient.InNamespace(defaultHelmReleaseNamespace),
		ctrlclient.MatchingLabels{"owner": "helm"},
	)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("No helm release secrets found in namespace", "namespace", defaultHelmReleaseNamespace)
			return nil
		}
		return fmt.Errorf("failed to list helm release secrets: %w", err)
	}

	// Search for secrets containing the legacy chart name
	var secretsToDelete []*corev1.Secret
	for i := range secretList.Items {
		secret := &secretList.Items[i]
		// Check if this secret contains a release with chart name "nutanix-k8s-agent"
		if n.isLegacyHelmRelease(secret) {
			secretsToDelete = append(secretsToDelete, secret)
		}
	}

	if len(secretsToDelete) == 0 {
		log.Info(
			"Legacy helm release not found",
			"chartName",
			legacyHelmChartName,
			"namespace",
			defaultHelmReleaseNamespace,
		)
		return nil
	}

	// Delete all matching helm release secrets
	for _, secret := range secretsToDelete {
		log.Info(
			"Deleting legacy helm release secret",
			"name",
			secret.Name,
			"namespace",
			secret.Namespace,
			"chartName",
			legacyHelmChartName,
		)
		if err := remoteClient.Delete(ctx, secret); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("Helm release secret already deleted", "name", secret.Name)
				continue
			}
			return fmt.Errorf("failed to delete helm release secret %q: %w", secret.Name, err)
		}
		log.Info("Successfully deleted legacy helm release secret", "name", secret.Name, "namespace", secret.Namespace)
	}

	return nil
}

// isLegacyHelmRelease checks if a helm release secret contains the legacy chart name "nutanix-k8s-agent"
// by examining the release data. We use chart name (not release name) since release names can vary.
func (n *DefaultKonnectorAgent) isLegacyHelmRelease(secret *corev1.Secret) bool {
	// Get the release data from the secret
	releaseData, ok := secret.Data["release"]
	if !ok {
		return false
	}

	// Decode the base64-encoded release data
	decoded, err := base64.StdEncoding.DecodeString(string(releaseData))
	if err != nil {
		return false
	}

	// Try to decompress with gzip (Helm v3 stores release data as gzip-compressed protobuf)
	var dataToSearch []byte
	gzipReader, err := gzip.NewReader(bytes.NewReader(decoded))
	if err == nil {
		// Successfully opened as gzip, read the decompressed data
		decompressed, err := io.ReadAll(gzipReader)
		_ = gzipReader.Close()
		if err == nil {
			dataToSearch = decompressed
		} else {
			// If decompression fails, fall back to searching the raw data
			dataToSearch = decoded
		}
	} else {
		// Not gzip-compressed, use the decoded data directly
		dataToSearch = decoded
	}

	// Search for the chart name in the data
	if strings.Contains(string(dataToSearch), legacyHelmChartName) {
		return true
	}

	// Try to parse as JSON (some helm versions might store it differently)
	var releaseInfo map[string]interface{}
	if err := json.Unmarshal(dataToSearch, &releaseInfo); err == nil {
		// Check chart.metadata.name which contains the chart name
		if chart, ok := releaseInfo["chart"].(map[string]interface{}); ok {
			if metadata, ok := chart["metadata"].(map[string]interface{}); ok {
				if name, ok := metadata["name"].(string); ok && name == legacyHelmChartName {
					return true
				}
			}
		}
	}

	return false
}

// ensureKonnectorAgentSecretFromPCGlobalSecret ensures that the konnectorAgent secret exists
// by creating it from PC global secrets if konnectorAgent is enabled but the secret doesn't exist.
// This is called during cluster upgrades to handle cases where the secret might be missing.
// Note: This function assumes konnectorAgent is already enabled (checked by the caller).
func (n *DefaultKonnectorAgent) ensureKonnectorAgentSecretFromPCGlobalSecret(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	varMap := variables.ClusterVariablesToVariablesMap(cluster.Spec.Topology.Variables)
	k8sAgentVar, err := variables.Get[apivariables.NutanixKonnectorAgent](
		varMap,
		n.variableName,
		n.variablePath...)
	if err != nil {
		// This should not happen if called from BeforeClusterUpgrade, but handle it gracefully
		return fmt.Errorf("failed to read Konnector Agent variable: %w", err)
	}

	// If credentials are already specified, check if the secret exists
	if k8sAgentVar.Credentials != nil && k8sAgentVar.Credentials.SecretRef.Name != "" {
		secretKey := ctrlclient.ObjectKey{
			Name:      k8sAgentVar.Credentials.SecretRef.Name,
			Namespace: cluster.Namespace,
		}
		secret := &corev1.Secret{}
		err := n.client.Get(ctx, secretKey, secret)
		if err == nil {
			// Secret exists, nothing to do
			log.Info("Konnector Agent secret already exists", "secret", secretKey)
			return nil
		}
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to check if konnectorAgent secret exists: %w", err)
		}
		// Secret doesn't exist, we'll create it from PC global secret below
		log.Info("Konnector Agent secret not found, will create from PC global secret", "secret", secretKey)
	}

	// Get PC global secret name from cluster configuration
	// Path: clusterConfig.nutanix.prismCentralEndpoint.credentials.secretRef.name
	clusterConfigVar, err := variables.Get[apivariables.ClusterConfigSpec](
		varMap,
		v1alpha1.ClusterConfigVariableName,
	)
	if err != nil {
		log.Info("Failed to read clusterConfig variable, cannot get PC global secret name", "error", err)
		return nil // Not an error, just skip secret creation
	}

	if clusterConfigVar.Nutanix == nil ||
		clusterConfigVar.Nutanix.PrismCentralEndpoint.Credentials.SecretRef.Name == "" {
		log.Info(
			"PC credentials secret reference not found in cluster configuration, cannot create konnectorAgent secret",
		)
		return nil // Not an error, just skip secret creation
	}

	pcGlobalSecretName := clusterConfigVar.Nutanix.PrismCentralEndpoint.Credentials.SecretRef.Name
	pcGlobalSecretKey := ctrlclient.ObjectKey{
		Name:      pcGlobalSecretName,
		Namespace: cluster.Namespace,
	}
	pcGlobalSecret := &corev1.Secret{}
	err = n.client.Get(ctx, pcGlobalSecretKey, pcGlobalSecret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("PC global secret not found, cannot create konnectorAgent secret", "secret", pcGlobalSecretKey)
			return nil // Not an error, just skip secret creation
		}
		return fmt.Errorf("failed to get PC global secret: %w", err)
	}

	// Parse credentials from PC global secret
	credentialsData, ok := pcGlobalSecret.Data["credentials"]
	if !ok {
		log.Info(
			"PC global secret does not contain 'credentials' key, cannot create konnectorAgent secret",
			"secret",
			pcGlobalSecretKey,
		)
		return nil // Not an error, just skip secret creation
	}

	pcCreds, err := prismcredentials.ParseCredentials(credentialsData)
	if err != nil {
		return fmt.Errorf("failed to parse PC credentials from global secret: %w", err)
	}

	// Determine the secret name for konnectorAgent
	konnectorAgentSecretName := fmt.Sprintf("%s-pc-credentials-for-konnector-agent", cluster.Name)
	if k8sAgentVar.Credentials != nil && k8sAgentVar.Credentials.SecretRef.Name != "" {
		konnectorAgentSecretName = k8sAgentVar.Credentials.SecretRef.Name
	}

	// Create the konnectorAgent secret
	konnectorAgentSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      konnectorAgentSecretName,
			Namespace: cluster.Namespace,
		},
		StringData: map[string]string{
			"username": pcCreds.Username,
			"password": pcCreds.Password,
		},
	}

	// Set owner reference to the cluster
	if err := ctrl.SetControllerReference(cluster, konnectorAgentSecret, n.client.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference on konnectorAgent secret: %w", err)
	}

	// Create or update the secret
	err = client.ServerSideApply(ctx, n.client, konnectorAgentSecret, client.ForceOwnership)
	if err != nil {
		return fmt.Errorf("failed to create konnectorAgent secret from PC global secret: %w", err)
	}

	log.Info("Successfully created konnectorAgent secret from PC global secret",
		"secret", ctrlclient.ObjectKeyFromObject(konnectorAgentSecret),
		"sourceSecret", pcGlobalSecretKey)

	return nil
}
