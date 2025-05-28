package nutanix

import (
	"fmt"

	prism "github.com/nutanix-cloud-native/prism-go-client"
	prismtypes "github.com/nutanix-cloud-native/prism-go-client/environment/types"
	prismv4 "github.com/nutanix-cloud-native/prism-go-client/v4"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func v4client(
	credentials *prismtypes.ApiCredentials,
	nutanixSpec *carenv1.NutanixSpec,
) (
	*prismv4.Client,
	error,
) {
	host, port, err := nutanixSpec.PrismCentralEndpoint.ParseURL()
	if err != nil {
		return nil, fmt.Errorf("failed to parse Prism Central endpoint: %w", err)
	}

	return prismv4.NewV4Client(prism.Credentials{
		Endpoint: fmt.Sprintf("%s:%d", host, port),
		Username: credentials.Username,
		Password: credentials.Password,
		Insecure: nutanixSpec.PrismCentralEndpoint.Insecure,
	})
}
