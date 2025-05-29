package nutanix

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	prismcredentials "github.com/nutanix-cloud-native/prism-go-client/environment/credentials"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

const credentialsSecretDataKey = "credentials"

func getCredentials(
	ctx context.Context,
	client ctrlclient.Client,
	cluster *clusterv1.Cluster,
	prismCentralEndpointSpec *carenv1.NutanixPrismCentralEndpointSpec,
) (*prismgoclient.Credentials, []preflight.Cause) {
	if prismCentralEndpointSpec == nil {
		return nil, []preflight.Cause{
			{
				Message: "Prism Central endpoint specification is missing",
				Field:   "cluster.spec.topology.variables[.name=clusterConfig].nutanix.prismCentralEndpoint",
			},
		}
	}

	if prismCentralEndpointSpec.Credentials.SecretRef.Name == "" {
		return nil, []preflight.Cause{
			{
				Message: "Prism Central credentials secret reference is missing the name",
				Field:   "cluster.spec.topology.variables[.name=clusterConfig].nutanix.prismCentralEndpoint.credentials.secretRef.name",
			},
		}
	}

	credentialsSecret := &corev1.Secret{}
	if err := client.Get(
		ctx,
		types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      prismCentralEndpointSpec.Credentials.SecretRef.Name,
		},
		credentialsSecret,
	); err != nil {
		return nil, []preflight.Cause{
			{
				Message: fmt.Sprintf("failed to get the credentials Secret: %s", err),
				Field:   "cluster.spec.topology.variables[.name=clusterConfig].nutanix.prismCentralEndpoint.credentials.secretRef",
			},
		}
	}

	if len(credentialsSecret.Data) == 0 {
		return nil, []preflight.Cause{
			{
				Message: fmt.Sprintf("credentials Secret has no data"),
				Field:   "cluster.spec.topology.variables[.name=clusterConfig].nutanix.prismCentralEndpoint.credentials.secretRef",
			},
		}
	}

	data, ok := credentialsSecret.Data[credentialsSecretDataKey]
	if !ok {
		return nil, []preflight.Cause{
			{
				Message: fmt.Sprintf("credentials Secret data is missing the key '%s'", credentialsSecretDataKey),
				Field:   "cluster.spec.topology.variables[.name=clusterConfig].nutanix.prismCentralEndpoint.credentials.secretRef",
			},
		}
	}

	usernamePassword, err := prismcredentials.ParseCredentials(data)
	if err != nil {
		return nil, []preflight.Cause{
			{
				Message: fmt.Sprintf("failed to parse credentials from Secret: %s", err),
				Field:   "cluster.spec.topology.variables[.name=clusterConfig].nutanix.prismCentralEndpoint.credentials.secretRef",
			},
		}
	}

	host, port, err := prismCentralEndpointSpec.ParseURL()
	if err != nil {
		return nil, []preflight.Cause{
			{
				Message: fmt.Sprintf("failed to parse Prism Central URL: %s", err),
				Field:   "cluster.spec.topology.variables[.name=clusterConfig].nutanix.prismCentralEndpoint.url",
			},
		}
	}

	return &prismgoclient.Credentials{
		Endpoint: fmt.Sprintf("%s:%d", host, port),
		Username: usernamePassword.Username,
		Password: usernamePassword.Password,
		Insecure: prismCentralEndpointSpec.Insecure,
	}, nil
}
