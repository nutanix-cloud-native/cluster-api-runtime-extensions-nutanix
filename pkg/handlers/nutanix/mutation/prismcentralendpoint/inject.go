// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package prismcentralendpoint

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"

	"github.com/nutanix-cloud-native/prism-go-client/environment/credentials"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capxv1 "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "prismCentralEndpoint"
)

type nutanixPrismCentralEndpoint struct {
	variableName      string
	variableFieldPath []string
}

func NewPatch() *nutanixPrismCentralEndpoint {
	return newNutanixPrismCentralEndpoint(
		clusterconfig.MetaVariableName,
		v1alpha1.NutanixVariableName,
		VariableName,
	)
}

func newNutanixPrismCentralEndpoint(
	variableName string,
	variableFieldPath ...string,
) *nutanixPrismCentralEndpoint {
	return &nutanixPrismCentralEndpoint{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *nutanixPrismCentralEndpoint) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	clusterKey client.ObjectKey,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	prismCentralEndpointVar, found, err := variables.Get[v1alpha1.NutanixPrismCentralEndpointSpec](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		return err
	}
	if !found {
		log.V(5).Info("Nutanix PrismCentralEndpoint variable not defined")
		return nil
	}

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		prismCentralEndpointVar,
	)

	return patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.InfrastructureCluster(capxv1.GroupVersion.Version, "NutanixClusterTemplate"),
		log,
		func(obj *capxv1.NutanixClusterTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("setting prismCentralEndpoint in NutanixCluster spec")

			var address string
			var port int32
			address, port, err = parsePrismCentralURL(prismCentralEndpointVar.URL)
			if err != nil {
				return err
			}
			prismCentral := &credentials.NutanixPrismEndpoint{
				Address:  address,
				Port:     port,
				Insecure: prismCentralEndpointVar.Insecure,
				CredentialRef: &credentials.NutanixCredentialReference{
					Kind: credentials.SecretKind,
					Name: prismCentralEndpointVar.Credentials.Name,
					// Assume the secret is in the same namespace as Cluster
					Namespace: clusterKey.Namespace,
				},
			}
			additionalTrustBundle := ptr.Deref(prismCentralEndpointVar.AdditionalTrustBundle, "")
			if additionalTrustBundle != "" {
				var decoded []byte
				decoded, err = base64.StdEncoding.DecodeString(additionalTrustBundle)
				if err != nil {
					log.Error(err, "error decoding additional trust bundle")
					return fmt.Errorf("error decoding additional trust bundle: %w", err)
				}
				prismCentral.AdditionalTrustBundle = &credentials.NutanixTrustBundleReference{
					Kind: credentials.NutanixTrustBundleKindString,
					Data: string(decoded),
				}
				// TODO: Consider always setting Insecure to false when AdditionalTrustBundle is set.
				// But do it in a webhook and not hidden in this handler.
			}

			obj.Spec.Template.Spec.PrismCentral = prismCentral

			return nil
		},
	)
}

//nolint:gocritic // no need for named return values
func parsePrismCentralURL(in string) (string, int32, error) {
	var prismCentralURL *url.URL
	prismCentralURL, err := url.Parse(in)
	if err != nil {
		return "", -1, fmt.Errorf("error parsing Prism Central URL: %w", err)
	}

	hostname := prismCentralURL.Hostname()

	// return early with the default port if no port is specified
	if prismCentralURL.Port() == "" {
		return hostname, v1alpha1.DefaultPrismCentralPort, nil
	}

	port, err := strconv.ParseInt(prismCentralURL.Port(), 10, 32)
	if err != nil {
		return "", -1, fmt.Errorf("error converting port to int: %w", err)
	}

	return hostname, int32(port), nil
}
