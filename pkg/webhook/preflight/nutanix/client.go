package nutanix

import (
	"context"
	"fmt"

	prism "github.com/nutanix-cloud-native/prism-go-client"
	prismcredentials "github.com/nutanix-cloud-native/prism-go-client/environment/credentials"
	prismv4 "github.com/nutanix-cloud-native/prism-go-client/v4"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

func (n *Checker) v4client(
	ctx context.Context,
	client ctrlclient.Client,
	clusterNamespace string,
) (
	*prismv4.Client,
	error,
) {
	n.clientMutex.Lock()
	defer n.clientMutex.Unlock()
	if n.nutanixClient != nil {
		return n.nutanixClient, nil
	}

	clusterConfig, err := variables.UnmarshalClusterConfigVariable(n.cluster.Spec.Topology.Variables)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal cluster variable %q: %w", carenv1.ClusterConfigVariableName, err)
	}

	if clusterConfig.Nutanix == nil {
		return nil, fmt.Errorf("missing Nutanix configuration in cluster topology")
	}

	if clusterConfig.Nutanix.PrismCentralEndpoint.Credentials.SecretRef.Name == "" {
		return nil, fmt.Errorf("prism Central credentials reference SecretRef.Name has no value")
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
	})
}
