package nutanix

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	prismcredentials "github.com/nutanix-cloud-native/prism-go-client/environment/credentials"
	prismtypes "github.com/nutanix-cloud-native/prism-go-client/environment/types"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

const credentialsSecretDataKey = "credentials"

func getCredentials(
	ctx context.Context,
	client ctrlclient.Client,
	cluster *clusterv1.Cluster,
	nutanixSpec *carenv1.NutanixSpec,
) (*prismtypes.ApiCredentials, error) {
	if nutanixSpec.PrismCentralEndpoint.Credentials.SecretRef.Name == "" {
		return nil, fmt.Errorf("secretRef.name has no value")
	}

	credentialsSecret := &corev1.Secret{}
	if err := client.Get(
		ctx,
		types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      nutanixSpec.PrismCentralEndpoint.Credentials.SecretRef.Name,
		},
		credentialsSecret,
	); err != nil {
		return nil, fmt.Errorf("failed to get Secret: %w", err)
	}

	if len(credentialsSecret.Data) == 0 {
		return nil, fmt.Errorf(
			"the Secret %s/%s has no data",
			cluster.Namespace,
			nutanixSpec.PrismCentralEndpoint.Credentials.SecretRef.Name,
		)
	}

	data, ok := credentialsSecret.Data[credentialsSecretDataKey]
	if !ok {
		return nil, fmt.Errorf(
			"the Secret %s/%s has no data for key %s",
			cluster.Namespace,
			nutanixSpec.PrismCentralEndpoint.Credentials.SecretRef.Name,
			credentialsSecretDataKey,
		)
	}
	// Get username and password
	return prismcredentials.ParseCredentials(data)
}
