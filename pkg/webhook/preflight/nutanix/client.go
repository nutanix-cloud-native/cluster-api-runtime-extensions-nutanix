package nutanix

import (
	"context"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	prism "github.com/nutanix-cloud-native/prism-go-client"
	prismcredentials "github.com/nutanix-cloud-native/prism-go-client/environment/credentials"
	prismv4 "github.com/nutanix-cloud-native/prism-go-client/v4"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

// ClientGetter provides methods to create Prism Central clients.
// These methods are thread-safe and cache the results for efficiency.
type ClientGetter struct {
	client        ctrlclient.Client
	cluster       *clusterv1.Cluster
	nutanixClient *prismv4.Client
}

// V4 creates a new Prism V4 client for the Nutanix cluster using Prism Central credentials
// referenced in the clusterConfig. The client is cached for future use. The function returns an
// error if the credentials cannot be retrieved or if the Prism Central endpoint cannot be parsed.
func (g *ClientGetter) V4(ctx context.Context, clusterConfig *variables.ClusterConfigSpec) (*prismv4.Client, error) {
	return sync.OnceValues(func() (*prismv4.Client, error) {
		if clusterConfig == nil || clusterConfig.Nutanix == nil {
			return nil, fmt.Errorf("clusterConfig variable is nil or does not contain Nutanix config")
		}

		if clusterConfig.Nutanix.PrismCentralEndpoint.Credentials.SecretRef.Name == "" {
			return nil, fmt.Errorf("prism Central credentials reference SecretRef.Name has no value")
		}

		credentialsSecret := &corev1.Secret{}
		if err := g.client.Get(
			ctx,
			types.NamespacedName{
				Namespace: g.cluster.Namespace,
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

		nutanixClient, err := prismv4.NewV4Client(prism.Credentials{
			Endpoint: fmt.Sprintf("%s:%d", host, port),
			Username: credentials.Username,
			Password: credentials.Password,
			Insecure: clusterConfig.Nutanix.PrismCentralEndpoint.Insecure,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Prism V4 client: %w", err)
		}
		g.nutanixClient = nutanixClient
		return g.nutanixClient, nil
	})()
}
