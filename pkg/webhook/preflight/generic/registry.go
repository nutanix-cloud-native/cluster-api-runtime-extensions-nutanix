// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package generic

import (
	"context"
	"fmt"
	"regexp"

	"github.com/regclient/regclient"
	"github.com/regclient/regclient/config"
	"github.com/regclient/regclient/types/ref"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

var (
	registryMirrorVarPath    = "cluster.spec.topology.variables[.name=clusterConfig].value.globalImageRegistryMirror"
	mirrorURLValidationRegex = regexp.MustCompile(
		`^https?://`,
	) // in order to use regclient we need to pass just a hostname
	// this regex allows us to strip it so we can verify connectivity for this test.
)

type registyCheck struct {
	registryMirror *carenv1.GlobalImageRegistryMirror
	imageRegistry  *carenv1.ImageRegistry
	field          string
	kclient        ctrlclient.Client
	cluster        *clusterv1.Cluster
}

func (r *registyCheck) Name() string {
	return "RegistryCredentials"
}

func (r *registyCheck) Run(ctx context.Context) preflight.CheckResult {
	if r.registryMirror != nil {
		return r.checkRegistry(
			ctx,
			mirrorURLValidationRegex.ReplaceAllString(r.registryMirror.URL, ""),
			r.registryMirror.Credentials,
		)
	}
	if r.imageRegistry != nil {
		return r.checkRegistry(
			ctx,
			mirrorURLValidationRegex.ReplaceAllString(r.imageRegistry.URL, ""),
			r.imageRegistry.Credentials,
		)
	}
	return preflight.CheckResult{
		Allowed: true,
	}
}

func (r *registyCheck) checkRegistry(
	ctx context.Context,
	registryURL string,
	credentials *carenv1.RegistryCredentials,
) preflight.CheckResult {
	result := preflight.CheckResult{
		Allowed: false,
	}
	mirrorHost := config.Host{
		Name: registryURL,
	}
	if credentials != nil && credentials.SecretRef != nil {
		mirrorCredentialsSecret := &corev1.Secret{}
		err := r.kclient.Get(
			ctx,
			types.NamespacedName{
				Namespace: r.cluster.Namespace,
				Name:      r.registryMirror.Credentials.SecretRef.Name,
			},
			mirrorCredentialsSecret,
		)
		if err != nil {
			result.Allowed = false
			result.Error = true
			result.Causes = append(result.Causes,
				preflight.Cause{
					Message: fmt.Sprintf("failed to get Registry credentials Secret: %s", err),
					Field:   fmt.Sprintf("%s.credentials.secretRef", registryMirrorVarPath),
				},
			)
			return result
		}
		username, ok := mirrorCredentialsSecret.Data["username"]
		if !ok {
			result.Allowed = false
			result.Error = true
			result.Causes = append(result.Causes,
				preflight.Cause{
					Message: "failed to get username from Registry credentials Secret. secret must have field username.",
					Field:   fmt.Sprintf("%s.credentials.secretRef", registryMirrorVarPath),
				},
			)
			return result
		}
		password, ok := mirrorCredentialsSecret.Data["password"]
		if !ok {
			result.Allowed = false
			result.Error = true
			result.Causes = append(result.Causes,
				preflight.Cause{
					Message: "failed to get username from Registry credentials Secret. secret must have field password.",
					Field:   fmt.Sprintf("%s.credentials.secretRef", registryMirrorVarPath),
				},
			)
			return result
		}
		mirrorHost.User = string(username)
		mirrorHost.Pass = string(password)
		if caCert, ok := mirrorCredentialsSecret.Data["ca.crt"]; ok {
			mirrorHost.RegCert = string(caCert)
		}
	}
	rc := regclient.New(
		regclient.WithConfigHost(mirrorHost),
		regclient.WithUserAgent("regclient/example"),
	)
	mirrorRef, err := ref.NewHost(mirrorHost.Hostname)
	if err != nil {
		result.Allowed = false
		result.Error = true
		result.Causes = append(result.Causes,
			preflight.Cause{
				Message: "failed to create a client to verify registry configuration",
				Field:   registryMirrorVarPath,
			},
		)
		return result
	}
	_, err = rc.Ping(ctx, mirrorRef) // ping will return an error for anything that's not 200
	if err != nil {
		result.Allowed = false
		result.Error = true
		result.Causes = append(result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf("failed to ping registry %s with err: %s", mirrorHost.Hostname, err.Error()),
				Field:   registryMirrorVarPath,
			},
		)
		return result
	}
	result.Allowed = true
	result.Error = false
	return result
}

func newRegistryCheck(
	cd *checkDependencies,
) []preflight.Check {
	checks := []preflight.Check{}
	if cd.genericClusterConfigSpec != nil &&
		cd.genericClusterConfigSpec.GlobalImageRegistryMirror != nil {
		checks = append(checks, &registyCheck{
			field:          "cluster.spec.topology.variables[.name=clusterConfig].value.globalImageRegistryMirror",
			kclient:        cd.kclient,
			registryMirror: cd.genericClusterConfigSpec.GlobalImageRegistryMirror,
			cluster:        cd.cluster,
		})
	}
	if cd.genericClusterConfigSpec != nil && len(cd.genericClusterConfigSpec.ImageRegistries) > 0 {
		for i, registry := range cd.genericClusterConfigSpec.ImageRegistries {
			checks = append(checks, &registyCheck{
				field: fmt.Sprintf(
					"cluster.spec.topology.variables[.name=clusterConfig].value.imageRegistries[%d]",
					i,
				),
				kclient:       cd.kclient,
				imageRegistry: &registry,
				cluster:       cd.cluster,
			})
		}
	}
	return checks
}
