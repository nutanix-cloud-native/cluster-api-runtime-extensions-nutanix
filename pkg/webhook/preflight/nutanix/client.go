package nutanix

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	prism "github.com/nutanix-cloud-native/prism-go-client"
	prismcredentials "github.com/nutanix-cloud-native/prism-go-client/environment/credentials"
	prismv4 "github.com/nutanix-cloud-native/prism-go-client/v4"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func v4client(ctx context.Context,
	client ctrlclient.Client,
	cluster *clusterv1.Cluster,
	nutanixSpec *carenv1.NutanixSpec,
) (
	*prismv4.Client,
	error,
) {
	if nutanixSpec == nil {
		return nil, fmt.Errorf("nutanixSpec is nil")
	}

	if nutanixSpec.PrismCentralEndpoint.Credentials.SecretRef.Name == "" {
		return nil, fmt.Errorf("prism Central credentials reference SecretRef.Name has no value")
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
		return nil, fmt.Errorf("failed to get Prism Central credentials Secret: %w", err)
	}

	// Get username and password
	credentials, err := prismcredentials.ParseCredentials(credentialsSecret.Data["credentials"])
	if err != nil {
		return nil, fmt.Errorf("failed to parse Prism Central credentials from Secret: %w", err)
	}

	host, port, err := nutanixSpec.PrismCentralEndpoint.ParseURL()
	if err != nil {
		return nil, fmt.Errorf("failed to parse Prism Central endpoint: %w", err)
	}

	nutanixClient, err := prismv4.NewV4Client(prism.Credentials{
		Endpoint: fmt.Sprintf("%s:%d", host, port),
		Username: credentials.Username,
		Password: credentials.Password,
		Insecure: nutanixSpec.PrismCentralEndpoint.Insecure,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Prism V4 client: %w", err)
	}

	return nutanixClient, nil
}
