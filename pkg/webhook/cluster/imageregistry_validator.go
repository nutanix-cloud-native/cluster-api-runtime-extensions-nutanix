// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	v1 "k8s.io/api/admission/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

const (
	httpScheme = "http"
	caCertKey  = "ca.crt"
)

type imageRegistryValidator struct {
	client  ctrlclient.Client
	decoder admission.Decoder
}

func NewImageRegistryValidator(
	client ctrlclient.Client, decoder admission.Decoder,
) *imageRegistryValidator {
	return &imageRegistryValidator{
		client:  client,
		decoder: decoder,
	}
}

func (i *imageRegistryValidator) Validator() admission.HandlerFunc {
	return i.validate
}

func (i *imageRegistryValidator) validate(
	ctx context.Context,
	req admission.Request,
) admission.Response {
	if req.Operation == v1.Delete {
		return admission.Allowed("")
	}

	cluster := &clusterv1.Cluster{}
	if err := i.decoder.Decode(req, cluster); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if !cluster.Spec.Topology.IsDefined() {
		return admission.Allowed("")
	}

	clusterConfig, err := variables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return admission.Denied(
			fmt.Errorf("failed to unmarshal cluster topology variable %q: %w",
				v1alpha1.ClusterConfigVariableName,
				err).Error(),
		)
	}

	if clusterConfig == nil {
		return admission.Allowed("")
	}

	namespace := cluster.Namespace
	if namespace == "" {
		namespace = req.Namespace
	}

	if err := validateImageRegistries(
		ctx,
		i.client,
		namespace,
		clusterConfig.GenericClusterConfigSpec,
	); err != nil {
		return admission.Denied(err.Error())
	}

	return admission.Allowed("")
}

func validateImageRegistries(
	ctx context.Context,
	client ctrlclient.Reader,
	namespace string,
	spec v1alpha1.GenericClusterConfigSpec,
) error {
	if spec.GlobalImageRegistryMirror != nil {
		if err := validateHTTPRegistryNoCACert(
			ctx,
			client,
			namespace,
			"clusterConfig.globalImageRegistryMirror",
			spec.GlobalImageRegistryMirror.URL,
			spec.GlobalImageRegistryMirror.Credentials,
		); err != nil {
			return err
		}
	}

	hostSchemes := make(map[string]string)
	for idx, reg := range spec.ImageRegistries {
		u, err := url.ParseRequestURI(reg.URL)
		if err != nil {
			// URL format is already validated by OpenAPI; skip here.
			continue
		}
		host := u.Host
		scheme := u.Scheme
		if existing, ok := hostSchemes[host]; ok && existing != scheme {
			return fmt.Errorf(
				"clusterConfig.imageRegistries[%d].url: conflicting scheme for host %q"+
					" (schemes %q and %q are used for the same host)",
				idx,
				host,
				existing,
				scheme,
			)
		}
		hostSchemes[host] = scheme

		if err := validateHTTPRegistryNoCACert(
			ctx,
			client,
			namespace,
			fmt.Sprintf("clusterConfig.imageRegistries[%d]", idx),
			reg.URL,
			reg.Credentials,
		); err != nil {
			return err
		}
	}
	return nil
}

func validateHTTPRegistryNoCACert(
	ctx context.Context,
	client ctrlclient.Reader,
	namespace, fieldPath, rawURL string,
	credentials *v1alpha1.RegistryCredentials,
) error {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil || u.Scheme != httpScheme {
		return nil
	}

	secret, err := handlersutils.SecretForImageRegistryCredentials(ctx, client, credentials, namespace)
	if err != nil {
		if err = ctrlclient.IgnoreNotFound(err); err != nil {
			return fmt.Errorf(
				"failed to get Secret %s/%s: %w",
				namespace,
				handlersutils.SecretNameForImageRegistryCredentials(credentials),
				err,
			)
		}
		return nil
	}
	if secret == nil {
		return nil
	}

	if _, ok := secret.Data[caCertKey]; ok {
		return fmt.Errorf(
			"%s.credentials.secretRef: Secret %q contains %q, "+
				"which cannot be used with an %q registry URL %q",
			fieldPath,
			secret.Name,
			caCertKey,
			httpScheme,
			rawURL,
		)
	}

	return nil
}
