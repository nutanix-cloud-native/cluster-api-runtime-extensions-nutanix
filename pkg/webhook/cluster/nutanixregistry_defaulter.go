// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	webhook "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook"
)

const (
	nutanixRegistryURL         = "https://registry-1.docker.io/nutanix"
	nutanixRegistryUsernameKey = "username"
	nutanixRegistryPasswordKey = "password"
)

// nutanixRegistrySecretName is the per-cluster Secret holding Nutanix Registry credentials.
func nutanixRegistrySecretName(clusterName string) string {
	return fmt.Sprintf("%s-dockerhub-nutanix-credentials", clusterName)
}

// nutanixRegistryDefaulter injects Nutanix Registry credentials - inherited from the
// management cluster into a workload cluster's clusterConfig imageRegistries at Cluster CREATE or
// UPDATE admission, before the KubeadmControlPlane (and therefore its Machines) are generated.
//
// It is a no-op unless the Cluster is a CAREN clusterConfig cluster and a management cluster with
// credentials can be resolved. Existing explicit registry entries are preserved.
type nutanixRegistryDefaulter struct {
	client  ctrlclient.Client
	decoder admission.Decoder
}

func NewNutanixRegistryDefaulter(
	client ctrlclient.Client, decoder admission.Decoder,
) *nutanixRegistryDefaulter {
	return &nutanixRegistryDefaulter{client: client, decoder: decoder}
}

func (d *nutanixRegistryDefaulter) Defaulter() admission.HandlerFunc {
	return d.defaulter
}

func (d *nutanixRegistryDefaulter) defaulter(
	ctx context.Context,
	req admission.Request,
) admission.Response {
	cluster := &clusterv1.Cluster{}
	if err := d.decoder.Decode(req, cluster); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if err := d.mutateCluster(ctx, req, cluster); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	marshaledCluster, err := json.Marshal(cluster)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledCluster)
}

func (d *nutanixRegistryDefaulter) mutateCluster(
	ctx context.Context,
	req admission.Request,
	cluster *clusterv1.Cluster,
) error {
	log := ctrl.LoggerFrom(ctx).WithName("cluster/dockerhub-nutanix")

	if req.Operation != admissionv1.Create && req.Operation != admissionv1.Update {
		return nil
	}
	if cluster.GetDeletionTimestamp() != nil || !cluster.Spec.Topology.IsDefined() {
		return nil
	}
	// TODO (nilay): Remove this once we move to 2.20.
	if req.Operation == admissionv1.Update {
		is219, err := webhook.IsNKP219ClusterClass(cluster.Spec.Topology.ClassRef.Name)
		if err != nil {
			log.Error(err, "failed to parse NKP ClusterClass version")
			return nil
		}
		if !is219 {
			return nil
		}
	}

	if cluster.Name == "" {
		return nil
	}

	clusterConfig, err := variables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return fmt.Errorf("failed to unmarshal clusterConfig for docker.io/nutanix defaulting: %w", err)
	}
	if clusterConfig == nil {
		// Not a CAREN clusterConfig cluster.
		return nil
	}

	secretName := nutanixRegistrySecretName(cluster.Name)
	creds, err := d.resolveManagementCredentials(ctx, cluster)
	if err != nil {
		return err
	}
	if creds == nil {
		// No management cluster to inherit from, this Cluster IS the management cluster, or the
		// management cluster has no credentials to inherit (air-gapped / mirror-only).
		return nil
	}

	// Never create backing objects on a dry-run admission request.
	if !ptr.Deref(req.DryRun, false) {
		if err := d.ensureSecret(
			ctx, cluster.Namespace, secretName, cluster.Name, creds.username, creds.password,
		); err != nil {
			return err
		}
	}

	if hasDockerHubNutanixRegistry(clusterConfig.ImageRegistries) {
		return nil
	}
	appendDockerHubNutanixRegistry(clusterConfig, secretName)
	updatedVar, err := variables.MarshalToClusterVariable(carenv1.ClusterConfigVariableName, clusterConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal defaulted clusterConfig: %w", err)
	}
	cluster.Spec.Topology.Variables = variables.UpdateClusterVariable(
		updatedVar, cluster.Spec.Topology.Variables,
	)

	log.V(1).Info("inherited docker.io/nutanix credentials from management cluster into clusterConfig",
		"cluster", cluster.Name, "namespace", cluster.Namespace, "secret", secretName,
		"dryRun", ptr.Deref(req.DryRun, false))
	return nil
}

// hasDockerHubNutanixRegistry reports whether the clusterConfig already declares the
// docker.io/nutanix imageRegistries entry.
func hasDockerHubNutanixRegistry(registries []carenv1.ImageRegistry) bool {
	for i := range registries {
		if registries[i].URL == nutanixRegistryURL {
			return true
		}
	}
	return false
}

// appendDockerHubNutanixRegistry adds the docker.io/nutanix imageRegistries entry backed by
// secretName to the clusterConfig.
func appendDockerHubNutanixRegistry(clusterConfig *variables.ClusterConfigSpec, secretName string) {
	clusterConfig.ImageRegistries = append(clusterConfig.ImageRegistries, carenv1.ImageRegistry{
		URL: nutanixRegistryURL,
		Credentials: &carenv1.RegistryCredentials{
			SecretRef: &carenv1.LocalObjectReference{Name: secretName},
		},
	})
}

// inheritedDockerHubNutanixCredentials holds the credentials inherited from the management cluster.
type inheritedDockerHubNutanixCredentials struct {
	username string
	password string
}

// resolveManagementCredentials resolves the management cluster from node annotations and reads its
// docker.io/nutanix credentials. It returns nil (with a nil error) when there is nothing to inherit:
// no management cluster can be resolved, the workload Cluster is itself the management cluster, or
// the management cluster carries no credentials.
func (d *nutanixRegistryDefaulter) resolveManagementCredentials(
	ctx context.Context,
	workload *clusterv1.Cluster,
) (*inheritedDockerHubNutanixCredentials, error) {
	managementKey, err := d.managementClusterKey(ctx)
	if err != nil {
		return nil, err
	}
	if managementKey.Name == "" {
		return nil, nil
	}
	// The management cluster is the source of truth; it never inherits from itself.
	if managementKey.Namespace == workload.Namespace && managementKey.Name == workload.Name {
		return nil, nil
	}

	secret := &corev1.Secret{}
	err = d.client.Get(ctx, types.NamespacedName{
		Namespace: managementKey.Namespace,
		Name:      nutanixRegistrySecretName(managementKey.Name),
	}, secret)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read management docker.io/nutanix credentials: %w", err)
	}

	username := string(secret.Data[nutanixRegistryUsernameKey])
	password := string(secret.Data[nutanixRegistryPasswordKey])
	if username == "" && password == "" {
		return nil, nil
	}
	return &inheritedDockerHubNutanixCredentials{username: username, password: password}, nil
}

// managementClusterKey resolves the management CAPI Cluster's namespace/name from the CAPI
// annotations that the management cluster's own Machines stamp onto its Nodes.
func (d *nutanixRegistryDefaulter) managementClusterKey(ctx context.Context) (types.NamespacedName, error) {
	nodes := &corev1.NodeList{}
	if err := d.client.List(ctx, nodes); err != nil {
		return types.NamespacedName{}, fmt.Errorf("failed to list Nodes to resolve management cluster: %w", err)
	}
	for i := range nodes.Items {
		annotations := nodes.Items[i].Annotations
		name := annotations[clusterv1.ClusterNameAnnotation]
		if name != "" {
			return types.NamespacedName{
				Namespace: annotations[clusterv1.ClusterNamespaceAnnotation],
				Name:      name,
			}, nil
		}
	}
	return types.NamespacedName{}, nil
}

// ensureSecret creates the per-cluster docker.io/nutanix Secret if it does not already exist. It
// never updates an existing Secret, so per-cluster credentials (explicit at create, or set by a
// Day-2 rotation) are preserved.
func (d *nutanixRegistryDefaulter) ensureSecret(
	ctx context.Context,
	namespace, secretName, clusterName, username, password string,
) error {
	err := d.client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: secretName}, &corev1.Secret{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get docker.io/nutanix Secret %s/%s: %w", namespace, secretName, err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels:    map[string]string{clusterv1.ClusterNameLabel: clusterName},
		},
		StringData: map[string]string{
			nutanixRegistryUsernameKey: username,
			nutanixRegistryPasswordKey: password,
		},
		Type: corev1.SecretTypeOpaque,
	}
	if err := d.client.Create(ctx, secret); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create docker.io/nutanix Secret %s/%s: %w", namespace, secretName, err)
	}
	return nil
}
