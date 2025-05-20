// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	prismcredentials "github.com/nutanix-cloud-native/prism-go-client/environment/credentials"
	prismv3 "github.com/nutanix-cloud-native/prism-go-client/v3"
	prismv4 "github.com/nutanix-cloud-native/prism-go-client/v4"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

const credentialsSecretDataKey = "credentials"

func (n *nutanixChecker) initCredentialsCheck(ctx context.Context) preflight.Check {
	n.log.V(5).Info("Initializing Nutanix credentials check")

	result := preflight.CheckResult{
		Name:    "NutanixCredentials",
		Allowed: true,
	}

	if n.nutanixClusterConfigSpec == nil && len(n.nutanixWorkerNodeConfigSpecByMachineDeploymentName) == 0 {
		// If there is no Nutanix configuration at all, the credentials check is not needed.
		return func(ctx context.Context) preflight.CheckResult {
			return result
		}
	}

	// There is some Nutanix configuration, so the credentials check is needed.
	// However, the credentials configuration is missing, so we cannot perform the check.
	if n.nutanixClusterConfigSpec == nil || n.nutanixClusterConfigSpec.Nutanix == nil {
		result.Allowed = false
		result.Error = true
		result.Causes = append(result.Causes,
			preflight.Cause{
				Message: "Nutanix cluster configuration is not defined in the cluster spec",
				Field:   "cluster.spec.topology.variables[.name=clusterConfig].nutanix",
			},
		)
		return func(ctx context.Context) preflight.CheckResult {
			return result
		}
	}

	// Get the credentials data in order to initialize the credentials and clients.
	prismCentralEndpointSpec := n.nutanixClusterConfigSpec.Nutanix.PrismCentralEndpoint

	host, port, err := prismCentralEndpointSpec.ParseURL()
	if err != nil {
		// Should not happen if the cluster passed CEL validation rules.
		result.Allowed = false
		result.Error = true
		result.Causes = append(result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf("failed to parse Prism Central endpoint URL: %s", err),
				Field:   "cluster.spec.topology.variables[.name=clusterConfig].nutanix.prismCentralEndpoint.url",
			},
		)
		return func(ctx context.Context) preflight.CheckResult {
			return result
		}
	}

	credentialsSecret := &corev1.Secret{}
	err = n.kclient.Get(
		ctx,
		types.NamespacedName{
			Namespace: n.cluster.Namespace,
			Name:      prismCentralEndpointSpec.Credentials.SecretRef.Name,
		},
		credentialsSecret,
	)
	if err != nil {
		result.Allowed = false
		result.Error = true
		result.Causes = append(result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf("failed to get Prism Central credentials Secret: %s", err),
				Field:   "cluster.spec.topology.variables[.name=clusterConfig].nutanix.prismCentralEndpoint.credentials.secretRef",
			},
		)
		return func(ctx context.Context) preflight.CheckResult {
			return result
		}
	}

	if len(credentialsSecret.Data) == 0 {
		result.Allowed = false
		result.Error = true
		result.Causes = append(result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf(
					"credentials Secret '%s' is empty",
					prismCentralEndpointSpec.Credentials.SecretRef.Name,
				),
				Field: "cluster.spec.topology.variables[.name=clusterConfig].nutanix.prismCentralEndpoint.credentials.secretRef",
			},
		)
		return func(ctx context.Context) preflight.CheckResult {
			return result
		}
	}

	data, ok := credentialsSecret.Data[credentialsSecretDataKey]
	if !ok {
		result.Allowed = false
		result.Error = true
		result.Causes = append(result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf(
					"credentials Secret '%s' does not contain key '%s'",
					prismCentralEndpointSpec.Credentials.SecretRef.Name,
					credentialsSecretDataKey,
				),
				Field: "cluster.spec.topology.variables[.name=clusterConfig].nutanix.prismCentralEndpoint.credentials.secretRef",
			},
		)
		return func(ctx context.Context) preflight.CheckResult {
			return result
		}
	}

	usernamePassword, err := prismcredentials.ParseCredentials(data)
	if err != nil {
		result.Allowed = false
		result.Error = true
		result.Causes = append(result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf("failed to parse Prism Central credentials: %s", err),
				Field:   "cluster.spec.topology.variables[.name=clusterConfig].nutanix.prismCentralEndpoint.credentials",
			},
		)
		return func(ctx context.Context) preflight.CheckResult {
			return result
		}
	}

	if !result.Allowed || result.Error {
		// If any error has happened, we should not try to initialize the credentials or clients, so we return early.
		return func(ctx context.Context) preflight.CheckResult {
			return result
		}
	}

	// Initialize the credentials.
	n.credentials = prismgoclient.Credentials{
		Endpoint: fmt.Sprintf("%s:%d", host, port),
		URL:      fmt.Sprintf("https://%s:%d", host, port),
		Username: usernamePassword.Username,
		Password: usernamePassword.Password,
	}
	if prismCentralEndpointSpec.Insecure {
		n.credentials.Insecure = true
		n.credentials.URL = fmt.Sprintf("http://%s:%d", host, port)
	}

	// Initialize the clients.
	n.v4client, err = prismv4.NewV4Client(n.credentials)
	if err != nil {
		result.Allowed = false
		result.Error = true
		result.Causes = append(result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf("failed to initialize Nutanix V4 client: %s", err),
				Field:   "cluster.spec.topology.variables[.name=clusterConfig].nutanix.prismCentralEndpoint.credentials",
			},
		)
	}

	n.v3client, err = prismv3.NewV3Client(n.credentials)
	if err != nil {
		result.Allowed = false
		result.Error = true
		result.Causes = append(result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf("failed to initialize Nutanix V3 client: %s", err),
				Field:   "cluster.spec.topology.variables[.name=clusterConfig].nutanix.prismCentralEndpoint.credentials",
			},
		)
	}
	_, err = n.v3client.V3.GetCurrentLoggedInUser(ctx)
	if err != nil {
		result.Allowed = false
		result.Error = true
		result.Causes = append(result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf("failed to validate credentials using the v3 API client: %s", err),
				Field:   "cluster.spec.topology.variables[.name=clusterConfig].nutanix.prismCentralEndpoint.credentials",
			},
		)
	}

	return func(ctx context.Context) preflight.CheckResult {
		return result
	}
}
