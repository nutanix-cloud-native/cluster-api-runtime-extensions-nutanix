// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

func newCredentialsCheck(
	ctx context.Context,
	nclientFactory func(prismgoclient.Credentials) (client, error),
	cd *checkDependencies,
) preflight.Check {
	cd.log.V(5).Info("Initializing Nutanix credentials check")

	credentialsCheck := &credentialsCheck{
		result: preflight.CheckResult{
			Allowed: true,
		},
	}

	if cd.nutanixClusterConfigSpec == nil && len(cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName) == 0 {
		// If there is no Nutanix configuration at all, the credentials check is not needed.
		return credentialsCheck
	}

	// There is some Nutanix configuration, so the credentials check is needed.
	// However, the credentials configuration is missing, so we cannot perform the check.
	if cd.nutanixClusterConfigSpec == nil || cd.nutanixClusterConfigSpec.Nutanix == nil {
		credentialsCheck.result.Allowed = false
		credentialsCheck.result.Causes = append(credentialsCheck.result.Causes,
			preflight.Cause{
				Message: "Nutanix cluster configuration is not defined in the cluster spec",
				Field:   "$.spec.topology.variables[?@.name==\"clusterConfig\"].value.nutanix",
			},
		)
		return credentialsCheck
	}

	// Get the credentials data in order to initialize the credentials and clients.
	prismCentralEndpointSpec := cd.nutanixClusterConfigSpec.Nutanix.PrismCentralEndpoint

	host, port, err := prismCentralEndpointSpec.ParseURL()
	if err != nil {
		// Should not happen if the cluster passed CEL validation rules.
		credentialsCheck.result.Allowed = false
		credentialsCheck.result.Causes = append(credentialsCheck.result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf("Failed to parse Prism Central endpoint URL: %s", err),
				Field: "$.spec.topology.variables[?@.name==\"clusterConfig\"]" +
					".value.nutanix.prismCentralEndpoint.url",
			},
		)
		return credentialsCheck
	}

	credentialsSecret := &corev1.Secret{}
	err = cd.kclient.Get(
		ctx,
		types.NamespacedName{
			Namespace: cd.cluster.Namespace,
			Name:      prismCentralEndpointSpec.Credentials.SecretRef.Name,
		},
		credentialsSecret,
	)
	if apierrors.IsNotFound(err) {
		credentialsCheck.result.Allowed = false
		credentialsCheck.result.InternalError = false
		credentialsCheck.result.Causes = append(credentialsCheck.result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf("Prism Central credentials Secret %q not found",
					prismCentralEndpointSpec.Credentials.SecretRef.Name),
				Field: "$.spec.topology.variables[?@.name==\"clusterConfig\"]" +
					".value.nutanix.prismCentralEndpoint.credentials.secretRef",
			},
		)
		return credentialsCheck
	}
	if err != nil {
		credentialsCheck.result.Allowed = false
		credentialsCheck.result.InternalError = true
		credentialsCheck.result.Causes = append(credentialsCheck.result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf("Failed to get Prism Central credentials Secret %q: %s",
					prismCentralEndpointSpec.Credentials.SecretRef.Name,
					err,
				),
				Field: "$.spec.topology.variables[?@.name==\"clusterConfig\"]" +
					".value.nutanix.prismCentralEndpoint.credentials.secretRef",
			},
		)
		return credentialsCheck
	}

	if len(credentialsSecret.Data) == 0 {
		credentialsCheck.result.Allowed = false
		credentialsCheck.result.Causes = append(credentialsCheck.result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf(
					"Credentials Secret %q is empty",
					prismCentralEndpointSpec.Credentials.SecretRef.Name,
				),
				Field: "$.spec.topology.variables[?@.name==\"clusterConfig\"]" +
					".value.nutanix.prismCentralEndpoint.credentials.secretRef",
			},
		)
		return credentialsCheck
	}

	data, ok := credentialsSecret.Data[credentialsSecretDataKey]
	if !ok {
		credentialsCheck.result.Allowed = false
		credentialsCheck.result.Causes = append(credentialsCheck.result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf(
					"Credentials Secret %q does not contain key %q",
					prismCentralEndpointSpec.Credentials.SecretRef.Name,
					credentialsSecretDataKey,
				),
				Field: "$.spec.topology.variables[?@.name==\"clusterConfig\"]" +
					".value.nutanix.prismCentralEndpoint.credentials.secretRef",
			},
		)
		return credentialsCheck
	}

	usernamePassword, err := prismcredentials.ParseCredentials(data)
	if err != nil {
		credentialsCheck.result.Allowed = false
		credentialsCheck.result.Causes = append(credentialsCheck.result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf("Failed to parse Prism Central credentials: %s", err),
				Field: "$.spec.topology.variables[?@.name==\"clusterConfig\"]" +
					".value.nutanix.prismCentralEndpoint.credentials.secretRef",
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
	nclient, err := nclientFactory(credentials)
	if err != nil {
		credentialsCheck.result.Allowed = false
		credentialsCheck.result.InternalError = true
		credentialsCheck.result.Causes = append(credentialsCheck.result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf("Failed to initialize Nutanix client: %s", err),
				Field: "$.spec.topology.variables[?@.name==\"clusterConfig\"]" +
					".value.nutanix.prismCentralEndpoint.credentials.secretRef",
			},
		)
		return credentialsCheck
	}

	// Validate the credentials using an API call.
	_, err = nclient.GetCurrentLoggedInUser(ctx)
	if err == nil {
		// We initialized both clients, and verified the credentials using the v3 client.
		cd.nclient = nclient
		return credentialsCheck
	}

	if strings.Contains(err.Error(), "invalid Nutanix credentials") {
		credentialsCheck.result.Allowed = false
		credentialsCheck.result.Causes = append(credentialsCheck.result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf("Failed to validate credentials using the v3 API client: %s", err),
				Field: "$.spec.topology.variables[?@.name==\"clusterConfig\"]" +
					".value.nutanix.prismCentralEndpoint.credentials.secretRef",
			},
		)
		return credentialsCheck
	}

	credentialsCheck.result.Allowed = false
	credentialsCheck.result.InternalError = true
	credentialsCheck.result.Causes = append(credentialsCheck.result.Causes,
		preflight.Cause{
			Message: fmt.Sprintf("Failed to validate credentials using the v3 API client: %s", err),
			Field:   "$.spec.topology.variables[?@.name==\"clusterConfig\"].value.nutanix.prismCentralEndpoint",
		},
	)
	return credentialsCheck
}
