// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package prismcentralendpoint

import (
	"context"
	"encoding/base64"
	"fmt"

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

			prismCentral := &credentials.NutanixPrismEndpoint{
				Address:  prismCentralEndpointVar.Host,
				Port:     prismCentralEndpointVar.Port,
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
			}

			// Always force insecure to false if additional trust bundle is provided.
			// This ensures that the trust bundle is actually used to validate the connection.
			if additionalTrustBundle != "" && prismCentral.Insecure {
				log.Info("AdditionalTrustBundle is provided, setting insecure to false")
				prismCentral.Insecure = false
			}

			obj.Spec.Template.Spec.PrismCentral = prismCentral

			return nil
		},
	)
}
