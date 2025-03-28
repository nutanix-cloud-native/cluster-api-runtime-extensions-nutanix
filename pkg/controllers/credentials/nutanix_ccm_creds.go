package credentials

import (
	"context"
	"fmt"

	credsv1alpha1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	ccmHandler "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/ccm/nutanix"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *CredentialsRequestReconciler) reconcileNutanixCCMCredentials(
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

	ccmSecretOnRemote := ccmHandler.NutanixCCMCredentialsSecret(
		credRequest.Spec.SecretRef.Name,
		rootUser,
		rootPassword,
		cluster,
	)

	if err := handlersutils.CreateSecretOnRemoteCluster(ctx, r.Client, ccmSecretOnRemote, cluster); err != nil {
		log.Error(err, "Failed to create secret", "SecretName", credRequest.Spec.SecretRef.Name)
		return err
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
