package credentials

import (
	"context"
	"fmt"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	credsv1alpha1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/mutation/prismcentralendpoint"
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
	rootSecret, err := r.GetRootCredentialSecret(
		ctx,
		string(credRequest.Spec.RootCredentialsKey),
		cluster,
	)
	if err != nil {
		return err
	}
	if rootSecret == nil {
		return fmt.Errorf("root secret not found for credentials secret: %s", credRequest.Name)
	}
	rootUser, rootPassword, _, err := decodeRootCredentialsBasicAuth(
		rootSecret,
		string(credRequest.Spec.RootCredentialsKey),
	)
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
	rootCredentialsKey string,
	cluster *clusterv1.Cluster,
) (*corev1.Secret, error) {
	log := ctrl.LoggerFrom(ctx)
	rootSecrets := &corev1.SecretList{}
	selector, err := metav1.LabelSelectorAsSelector(
		&metav1.LabelSelector{
			MatchLabels: map[string]string{
				credsv1alpha1.LabelRootSecretWatchKey: rootCredentialsKey,
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
	rootCredentialsValueFromCluster, err := r.GetRootCredentialValueFromCluster(ctx, cluster)
	if err != nil {
		return nil, err
	}
	filteredRootSecrets := []corev1.Secret{}
	for _, secret := range rootSecrets.Items {
		_, _, rootValueFromSecret, err := decodeRootCredentialsBasicAuth(
			&secret,
			rootCredentialsKey,
		)
		if err != nil {
			log.Error(err, "Failed to decode root secret", "SecretName", secret.Name)
			continue
		}
		if rootCredentialsValueFromCluster == rootValueFromSecret {
			filteredRootSecrets = append(filteredRootSecrets, secret)
		}
	}

	if len(filteredRootSecrets) != 1 {
		log.Info("unable to find root secret for the cluster", cluster.GetName())
		return nil, fmt.Errorf(
			"unable to find root secret  for the cluster.Ensure single root secret is present for: %s",
			cluster.GetName(),
		)
	}
	return &filteredRootSecrets[0], nil
}

func (r *CredentialsRequestReconciler) GetRootCredentialValueFromCluster(
	ctx context.Context,
	cluster *clusterv1.Cluster) (string, error) {

	if cluster.Spec.InfrastructureRef == nil {
		return "", fmt.Errorf("infrastructure ref not found for cluster: %s", cluster.GetName())
	}
	switch cluster.Spec.InfrastructureRef.Kind {
	case "NutanixCluster":
		prismCentralEndpointVar, err := variables.Get[v1alpha1.NutanixPrismCentralEndpointSpec](
			variables.ClusterVariablesToVariablesMap(cluster.Spec.Topology.Variables),
			v1alpha1.ClusterConfigVariableName,
			v1alpha1.NutanixVariableName,
			prismcentralendpoint.VariableName,
		)
		if err != nil {
			return "", err
		}
		return prismCentralEndpointVar.URL, nil
	default:
	}
	return "", fmt.Errorf(
		"unsupported infrastructure ref kind: %s",
		cluster.Spec.InfrastructureRef.Kind,
	)
}

func decodeRootCredentialsBasicAuth(
	secret *corev1.Secret,
	rootCredentialsKey string,
) (string, string, string, error) {
	username, ok := secret.Data["username"]
	if !ok {
		return "", "", "", fmt.Errorf("username field not found in secret")
	}

	password, ok := secret.Data["password"]
	if !ok {
		return "", "", "", fmt.Errorf("password field not found in secret")
	}

	rootValue, ok := secret.Data[rootCredentialsKey]
	if !ok {
		return "", "", "", fmt.Errorf("root value field not found in secret")
	}

	return string(username), string(password), string(rootValue), nil
}
