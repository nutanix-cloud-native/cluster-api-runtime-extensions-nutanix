package nutanix

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	prism "github.com/nutanix-cloud-native/prism-go-client"
	prismcredentials "github.com/nutanix-cloud-native/prism-go-client/environment/credentials"
	prismv4 "github.com/nutanix-cloud-native/prism-go-client/v4"

	carenvariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

func newV4Client(
	ctx context.Context,
	client ctrlclient.Client,
	clusterNamespace string,
	clusterConfig *carenvariables.ClusterConfigSpec,
) (
	*prismv4.Client,
	error,
) {
	if clusterConfig.Nutanix.PrismCentralEndpoint.Credentials.SecretRef.Name == "" {
		return nil, fmt.Errorf("Prism Central credentials reference SecretRef.Name has no value")
	}

	credentialsSecret := &corev1.Secret{}
	if err := client.Get(
		ctx,
		types.NamespacedName{
			Namespace: clusterNamespace,
			Name:      clusterConfig.Nutanix.PrismCentralEndpoint.Credentials.SecretRef.Name,
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

	host, port, err := clusterConfig.Nutanix.PrismCentralEndpoint.ParseURL()
	if err != nil {
		return nil, fmt.Errorf("failed to parse Prism Central endpoint: %w", err)
	}

	return prismv4.NewV4Client(prism.Credentials{
		Endpoint: fmt.Sprintf("%s:%d", host, port),
		Username: credentials.Username,
		Password: credentials.Password,
		Insecure: clusterConfig.Nutanix.PrismCentralEndpoint.Insecure,
		// TODO AdditionalTrustBundle
	})
}
