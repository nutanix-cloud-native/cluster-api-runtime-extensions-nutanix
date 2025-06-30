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
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

var registryMirrorVarPath = "cluster.spec.topology.variables[.name=clusterConfig].value.globalImageRegistryMirror"

type registryCheck struct {
	registryMirror        *carenv1.GlobalImageRegistryMirror
	imageRegistry         *carenv1.ImageRegistry
	field                 string
	kclient               ctrlclient.Client
	cluster               *clusterv1.Cluster
	regClientPingerGetter regClientPingerFactory
	log                   logr.Logger
}

func (r *registryCheck) Name() string {
	return "RegistryCredentials"
}

func (r *registryCheck) Run(ctx context.Context) preflight.CheckResult {
	if r.registryMirror != nil {
		return r.checkRegistry(
			ctx,
			r.registryMirror.URL,
			r.registryMirror.Credentials,
			r.regClientPingerGetter,
		)
	}
	if r.imageRegistry != nil {
		return r.checkRegistry(
			ctx,
			r.imageRegistry.URL,
			r.imageRegistry.Credentials,
			r.regClientPingerGetter,
		)
	}
	return preflight.CheckResult{
		Allowed: true,
	}
}

type regClientPinger interface {
	Ping(context.Context, ref.Ref) (ping.Result, error)
}

type regClientPingerFactory func(...regclient.Opt) regClientPinger

func defaultRegClientGetter(opts ...regclient.Opt) regClientPinger {
	return regclient.New(opts...)
}

func pingFailedReasonString(registryURL string, err error) string {
	return fmt.Sprintf("failed to ping registry %s with err: %s", registryURL, err.Error())
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
	registryURLParsed, err := url.ParseRequestURI(registryURL)
	if err != nil {
		result.Allowed = false
		result.Error = true
		result.Causes = append(result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf("failed to parse registry url %s with error : %s", registryURL, err),
				Field:   registryMirrorVarPath,
			},
		)
		return result
	}
	mirrorHost := config.Host{
		Name: registryURLParsed.Host,
	}
	if credentials != nil && credentials.SecretRef != nil {
		mirrorCredentialsSecret := &corev1.Secret{}
		err := r.kclient.Get(
			ctx,
			types.NamespacedName{
				Namespace: r.cluster.Namespace,
				Name:      credentials.SecretRef.Name,
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
		if ok {
			mirrorHost.User = string(username)
		}
		password, ok := mirrorCredentialsSecret.Data["password"]
		if ok {
			mirrorHost.Pass = string(password)
		}
		if caCert, ok := mirrorCredentialsSecret.Data["ca.crt"]; ok {
			mirrorHost.RegCert = string(caCert)
		}
	}
	rc := regClientGetter(
		regclient.WithConfigHost(mirrorHost),
		regclient.WithUserAgent("regclient/example"),
	)
	mirrorRef, err := ref.NewHost(registryURLParsed.Host)
	if err != nil {
		result.Allowed = false
		result.Error = true
		result.Causes = append(result.Causes,
			preflight.Cause{
				Message: fmt.Sprintf("failed to create a client to verify registry configuration %s", err.Error()),
				Field:   registryMirrorVarPath,
			},
		)
		return result
	}
	_, err = rc.Ping(ctx, mirrorRef) // ping will return an error for anything that's not 200
	if err != nil {
		result.Allowed = false
		result.Causes = append(result.Causes,
			preflight.Cause{
				Message: pingFailedReasonString(registryURL, err),
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
		checks = append(checks, &registryCheck{
			field:                 "cluster.spec.topology.variables[.name=clusterConfig].value.globalImageRegistryMirror",
			kclient:               cd.kclient,
			registryMirror:        cd.genericClusterConfigSpec.GlobalImageRegistryMirror.DeepCopy(),
			cluster:               cd.cluster,
			regClientPingerGetter: defaultRegClientGetter,
			log:                   cd.log,
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
				imageRegistry:         registry.DeepCopy(),
				cluster:               cd.cluster,
				regClientPingerGetter: defaultRegClientGetter,
				log:                   cd.log,
			})
		}
	}
	return checks
}
