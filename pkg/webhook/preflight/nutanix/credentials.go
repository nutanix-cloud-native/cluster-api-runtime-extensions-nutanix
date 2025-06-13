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

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

const credentialsSecretDataKey = "credentials"

type credentialsCheck struct {
	result preflight.CheckResult
}

func (c *credentialsCheck) Name() string {
	return "NutanixCredentials"
}

func (c *credentialsCheck) Run(_ context.Context) preflight.CheckResult {
	return c.result
}

func initCredentialsCheck(
	ctx context.Context,
	n *nutanixChecker,
) preflight.Check {
	n.log.V(5).Info("Initializing Nutanix credentials check")

	credentialsCheck := &credentialsCheck{
		result: preflight.CheckResult{
			Allowed: true,
		},
	}

	if n.nutanixClusterConfigSpec == nil && len(n.nutanixWorkerNodeConfigSpecByMachineDeploymentName) == 0 {
		// If there is no Nutanix configuration at all, the credentials check is not needed.
		return credentialsCheck
	}

	// There is some Nutanix configuration, so the credentials check is needed.
	// However, the credentials configuration is missing, so we cannot perform the check.
	if n.nutanixClusterConfigSpec == nil || n.nutanixClusterConfigSpec.Nutanix == nil {
		credentialsCheck.result.Allowed = false
		credentialsCheck.result.Error = true
		credentialsCheck.result.Causes = append(credentialsCheck.result.Causes,
			preflight.Cause{
				Message: "Nutanix cluster configuration is not defined in the cluster spec",
				Field:   "cluster.spec.topology.variables[.name=clusterConfig].nutanix",
			},
		)
		return credentialsCheck
	}

	// Get the credentials data in order to initialize the credentials and clients.
	prismCentralEndpointSpec := n.nutanixClusterConfigSpec.Nutanix.PrismCentralEndpoint

	host, port, err := prismCentralEndpointSpec.ParseURL()
	if err != nil {
		// Should not happen if the cluster passed CEL validation rules.
		credentialsCheck.result.Allowed = false
		credentialsCheck.result.Error = true
		credentialsCheck.result.Causes = append(credentialsCheck.result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf("failed to parse Prism Central endpoint URL: %s", err),
				Field:   "cluster.spec.topology.variables[.name=clusterConfig].nutanix.prismCentralEndpoint.url",
			},
		)
		return credentialsCheck
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
		credentialsCheck.result.Allowed = false
		credentialsCheck.result.Error = true
		credentialsCheck.result.Causes = append(credentialsCheck.result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf("failed to get Prism Central credentials Secret: %s", err),
				Field:   "cluster.spec.topology.variables[.name=clusterConfig].nutanix.prismCentralEndpoint.credentials.secretRef",
			},
		)
		return credentialsCheck
	}

	if len(credentialsSecret.Data) == 0 {
		credentialsCheck.result.Allowed = false
		credentialsCheck.result.Error = true
		credentialsCheck.result.Causes = append(credentialsCheck.result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf(
					"credentials Secret '%s' is empty",
					prismCentralEndpointSpec.Credentials.SecretRef.Name,
				),
				Field: "cluster.spec.topology.variables[.name=clusterConfig].nutanix.prismCentralEndpoint.credentials.secretRef",
			},
		)
		return credentialsCheck
	}

	data, ok := credentialsSecret.Data[credentialsSecretDataKey]
	if !ok {
		credentialsCheck.result.Allowed = false
		credentialsCheck.result.Error = true
		credentialsCheck.result.Causes = append(credentialsCheck.result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf(
					"credentials Secret '%s' does not contain key '%s'",
					prismCentralEndpointSpec.Credentials.SecretRef.Name,
					credentialsSecretDataKey,
				),
				Field: "cluster.spec.topology.variables[.name=clusterConfig].nutanix.prismCentralEndpoint.credentials.secretRef",
			},
		)
		return credentialsCheck
	}

	usernamePassword, err := prismcredentials.ParseCredentials(data)
	if err != nil {
		credentialsCheck.result.Allowed = false
		credentialsCheck.result.Error = true
		credentialsCheck.result.Causes = append(credentialsCheck.result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf("failed to parse Prism Central credentials: %s", err),
				Field:   "cluster.spec.topology.variables[.name=clusterConfig].nutanix.prismCentralEndpoint.credentials",
			},
		)
		return credentialsCheck
	}

	// Initialize the credentials.
	credentials := prismgoclient.Credentials{
		Endpoint: fmt.Sprintf("%s:%d", host, port),
		URL:      fmt.Sprintf("https://%s:%d", host, port),
		Username: usernamePassword.Username,
		Password: usernamePassword.Password,
		Insecure: prismCentralEndpointSpec.Insecure,
	}

	// Initialize the Nutanix client.
	nclient, err := n.nclientFactory(credentials)
	if err != nil {
		credentialsCheck.result.Allowed = false
		credentialsCheck.result.Error = true
		credentialsCheck.result.Causes = append(credentialsCheck.result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf("Failed to initialize Nutanix client: %s", err),
				Field:   "cluster.spec.topology.variables[.name=clusterConfig].nutanix.prismCentralEndpoint.credentials",
			},
		)
		return credentialsCheck
	}

	// Validate the credentials using an API call.
	_, err = nclient.GetCurrentLoggedInUser(ctx)
	if err != nil {
		credentialsCheck.result.Allowed = false
		credentialsCheck.result.Error = true
		credentialsCheck.result.Causes = append(credentialsCheck.result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf("Failed to validate credentials using the v3 API client. "+
					"The URL and/or credentials may be incorrect. (Error: %q)", err),
				Field: "cluster.spec.topology.variables[.name=clusterConfig].nutanix.prismCentralEndpoint",
			},
		)
		return credentialsCheck
	}

	// We initialized both clients, and verified the credentials using the v3 client.
	n.nclient = nclient

	return credentialsCheck
}
