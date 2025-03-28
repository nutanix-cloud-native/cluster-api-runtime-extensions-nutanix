package credentials

import (
	"context"
	"fmt"
	"slices"
	"strings"

	credsv1alpha1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8sClient "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	pcHandler "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/mutation/prismcentralendpoint"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *CredentialsRequestReconciler) reconcileNutanixClusterCredentials(
	ctx context.Context,
	credRequest *credsv1alpha1.CredentialsRequest,
	cluster *clusterv1.Cluster,
) error {
	log := ctrl.LoggerFrom(ctx)
	rootSecret, err := r.GetRootCredentialSecret(ctx, cluster)
	if err != nil {
		return err
	}
	if rootSecret == nil {
		return fmt.Errorf("root secret not found for credentials secret: %s", credRequest.Name)
	}
	rootUser, rootPassword, _, err := decodeNutanixRootCredentialsBasicAuth(rootSecret)
	if err != nil {
		return fmt.Errorf(
			"failed to decode basic auth for root secret %s: %w",
			rootSecret.Name,
			err,
		)
	}

	secret := pcHandler.NutanixPCCredentialsSecret(
		credRequest.Spec.SecretRef.Name,
		rootUser,
		rootPassword,
		cluster,
	)

	if err := k8sClient.ServerSideApply(ctx, r.Client, secret, client.ForceOwnership); err != nil {
		log.Error(err, "Failed to create secret", "SecretName", credRequest.Spec.SecretRef.Name)
		return err
	}
	// Add cluster ownership on the credentials secret
	err = handlersutils.EnsureClusterOwnerReferenceForObject(
		ctx,
		r.Client,
		corev1.TypedLocalObjectReference{
			Kind: "Secret",
			Name: credRequest.Spec.SecretRef.Name,
		},
		cluster,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to set owner reference for secret %s: %w",
			credRequest.Spec.SecretRef.Name,
			err,
		)
	}
	// Add credentials request ownership on the root secret
	err = handlersutils.EnsureTypedObjectOwnerReference(
		ctx,
		r.Client,
		corev1.TypedLocalObjectReference{
			Kind: rootSecret.Kind,
			Name: rootSecret.Name,
		},
		credRequest,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to set owner reference for root secret %s: %w",
			rootSecret.Name,
			err,
		)
	}
	return nil
}

func (r *CredentialsRequestReconciler) GetRootCredentialSecret(
	ctx context.Context,
	cluster *clusterv1.Cluster,
) (*corev1.Secret, error) {
	log := ctrl.LoggerFrom(ctx)

	//TODO: hardcode to nutanix for now. it cane be made dynamic in future using cluster.spec.infrastructureRef
	clusterInfra := "nutanix"
	rootSecrets := &corev1.SecretList{}
	selector, err := metav1.LabelSelectorAsSelector(
		&metav1.LabelSelector{
			MatchLabels: map[string]string{
				credsv1alpha1.LabelRootCredentialsInfra: clusterInfra,
			},
		},
	)
	if err != nil {
		log.Error(err, "Failed to create label selector for root secrets")
		return nil, err
	}
	if err := r.List(ctx, rootSecrets, &client.ListOptions{
		LabelSelector: selector,
	}); err != nil {
		log.Error(err, "Failed to list root secrets")
		return nil, err
	}
	if len(rootSecrets.Items) == 0 {
		log.Info("No root secrets found for the selector", "selector", selector)
		return nil, nil
	}
	clusterNamespace := cluster.GetNamespace()
	filteredRootSecrets := []corev1.Secret{}
	for _, secret := range rootSecrets.Items {
		allowedNamespacesList, ok := secret.Labels[credsv1alpha1.LabelRootCredentialsAllowedNamespaces]
		if !ok {
			continue
		}
		allowedNamespaces := strings.SplitN(allowedNamespacesList, ",", -1)
		if slices.Contains(allowedNamespaces, clusterNamespace) {
			filteredRootSecrets = append(filteredRootSecrets, secret)
		}

	}

	if len(filteredRootSecrets) == 0 {
		log.Info("no root credentials found for cluster", "cluster", cluster.GetName())
		return nil, nil
	}
	if len(filteredRootSecrets) > 1 {
		log.Info("multiple root credentials found. picking first one", "cluster", cluster.GetName())
	}
	return &filteredRootSecrets[0], nil
}

func decodeNutanixRootCredentialsBasicAuth(secret *corev1.Secret) (string, string, string, error) {
	username, ok := secret.Data["username"]
	if !ok {
		return "", "", "", fmt.Errorf("username field not found in secret")
	}

	password, ok := secret.Data["password"]
	if !ok {
		return "", "", "", fmt.Errorf("password field not found in secret")
	}

	pcEndpoint, ok := secret.Data["pcEndpoint"]
	if !ok {
		return "", "", "", fmt.Errorf("pc endpoint field not found in secret")
	}

	return string(username), string(password), string(pcEndpoint), nil
}
