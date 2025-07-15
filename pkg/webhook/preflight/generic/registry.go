// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package generic

import (
	"context"
	"fmt"
	"net/url"

	"github.com/go-logr/logr"
	"github.com/regclient/regclient"
	"github.com/regclient/regclient/config"
	"github.com/regclient/regclient/types/ping"
	"github.com/regclient/regclient/types/ref"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

type registryCheck struct {
	field                 string
	kclient               ctrlclient.Client
	cluster               *clusterv1.Cluster
	regClientPingerGetter regClientPingerFactory
	log                   logr.Logger

	registryURL string
	credentials *carenv1.RegistryCredentials
}

func (r *registryCheck) Name() string {
	return "Registry"
}

func (r *registryCheck) Run(ctx context.Context) preflight.CheckResult {
	return r.checkRegistry(
		ctx,
		r.registryURL,
		r.credentials,
		r.regClientPingerGetter,
	)
}

type regClientPinger interface {
	Ping(context.Context, ref.Ref) (ping.Result, error)
}

type regClientPingerFactory func(...regclient.Opt) regClientPinger

func defaultRegClientGetter(opts ...regclient.Opt) regClientPinger {
	return regclient.New(opts...)
}

func pingFailedMessage(registryURL string, err error) string {
	return fmt.Sprintf(
		"Failed to ping registry %q: %s. First verify that the management cluster can reach the registry, then retry.", ///nolint:lll // Message is long.
		registryURL,
		err.Error(),
	)
}

func (r *registryCheck) checkRegistry(
	ctx context.Context,
	registryURL string,
	credentials *carenv1.RegistryCredentials,
	regClientGetter regClientPingerFactory,
) preflight.CheckResult {
	result := preflight.CheckResult{
		Allowed: false,
	}
	if r.registryURL == "" {
		result.Allowed = true
		return result
	}
	registryURLParsed, err := url.ParseRequestURI(registryURL)
	if err != nil {
		result.Allowed = false
		result.InternalError = false
		result.Causes = append(result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf(
					"Failed to parse registry URL %q with error: %s. Review the Cluster.", ///nolint:lll // Message is long.",
					registryURL,
					err,
				),
				Field: r.field + ".url",
			},
		)
		return result
	}
	registryHost := config.Host{
		Name: registryURLParsed.Host,
	}
	if registryURLParsed.Scheme != "http" && registryURLParsed.Scheme != "https" {
		result.Allowed = false
		result.Causes = append(result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf(
					"The registry URL %q uses the scheme %q. This scheme is not supported. Use either the \"http\" or \"https\" scheme.", ///nolint:lll // Message is long.
					registryURL,
					registryURLParsed.Scheme,
				),
				Field: r.field + ".url",
			},
		)
		return result
	}
	if registryURLParsed.Scheme == "http" {
		registryHost.TLS = config.TLSDisabled
	}

	if credentials != nil && credentials.SecretRef != nil {
		credentialsSecret := &corev1.Secret{}
		err := r.kclient.Get(
			ctx,
			types.NamespacedName{
				Namespace: r.cluster.Namespace,
				Name:      credentials.SecretRef.Name,
			},
			credentialsSecret,
		)
		if apierrors.IsNotFound(err) {
			result.Allowed = false
			result.InternalError = false
			result.Causes = append(result.Causes,
				preflight.Cause{
					Message: fmt.Sprintf(
						"Registry credentials Secret %q not found. Create the Secret first, then create the Cluster.", ///nolint:lll // Message is long.
						credentials.SecretRef.Name,
					),
					Field: r.field + ".credentials.secretRef",
				},
			)
			return result
		}
		if err != nil {
			result.Allowed = false
			result.InternalError = true
			result.Causes = append(result.Causes,
				preflight.Cause{
					Message: fmt.Sprintf(
						"Failed to get Registry credentials Secret %q: %s. This is usually a temporary error. Please retry.", ///nolint:lll // Message is long.
						credentials.SecretRef.Name,
						err,
					),
					Field: r.field + ".credentials.secretRef",
				},
			)
			return result
		}
		username, ok := credentialsSecret.Data["username"]
		if ok {
			registryHost.User = string(username)
		}
		password, ok := credentialsSecret.Data["password"]
		if ok {
			registryHost.Pass = string(password)
		}
		if caCert, ok := credentialsSecret.Data["ca.crt"]; ok {
			registryHost.RegCert = string(caCert)
		}
	}
	rc := regClientGetter(
		regclient.WithConfigHost(registryHost),
		regclient.WithUserAgent("regclient/caren"),
	)
	_, err = rc.Ping(ctx,
		ref.Ref{
			// Because we ping the registry, we only need the "registry" part of the ref.
			Registry: registryURLParsed.Host,
			// The default scheme is "reg""
			Scheme: "reg",
		},
	)
	// Ping will return an error for anything that's not 200.
	if err != nil {
		result.Allowed = false
		result.Causes = append(result.Causes,
			preflight.Cause{
				Message: pingFailedMessage(registryURL, err),
				Field:   r.field,
			},
		)
		return result
	}
	result.Allowed = true
	result.InternalError = false
	return result
}

func newRegistryCheck(
	cd *checkDependencies,
) []preflight.Check {
	checks := []preflight.Check{}
	if cd.genericClusterConfigSpec != nil &&
		cd.genericClusterConfigSpec.GlobalImageRegistryMirror != nil {
		checks = append(checks, &registryCheck{
			field:                 "cluster.spec.topology.variables[.name=clusterConfig].value.globalImageRegistryMirror",
			kclient:               cd.kclient,
			cluster:               cd.cluster,
			regClientPingerGetter: defaultRegClientGetter,
			log:                   cd.log,
			registryURL:           cd.genericClusterConfigSpec.GlobalImageRegistryMirror.DeepCopy().URL,
			credentials:           cd.genericClusterConfigSpec.GlobalImageRegistryMirror.DeepCopy().Credentials,
		})
	}
	if cd.genericClusterConfigSpec != nil && len(cd.genericClusterConfigSpec.ImageRegistries) > 0 {
		for i := range cd.genericClusterConfigSpec.ImageRegistries {
			registry := cd.genericClusterConfigSpec.ImageRegistries[i]
			checks = append(checks, &registryCheck{
				field: fmt.Sprintf(
					"cluster.spec.topology.variables[.name=clusterConfig].value.imageRegistries[%d]",
					i,
				),
				kclient:               cd.kclient,
				cluster:               cd.cluster,
				regClientPingerGetter: defaultRegClientGetter,
				log:                   cd.log,
				registryURL:           registry.DeepCopy().URL,
				credentials:           registry.DeepCopy().Credentials,
			})
		}
	}
	return checks
}
