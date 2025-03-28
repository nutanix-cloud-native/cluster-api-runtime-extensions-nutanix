// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package prismcentralendpoint

import (
	"context"
	"encoding/base64"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/prism-go-client/environment/credentials"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "prismCentralEndpoint"
)

type nutanixPrismCentralEndpoint struct {
	cl                ctrlclient.Client
	variableName      string
	variableFieldPath []string
}

func NewPatch(client ctrlclient.Client) *nutanixPrismCentralEndpoint {
	return newNutanixPrismCentralEndpoint(
		client,
		v1alpha1.ClusterConfigVariableName,
		v1alpha1.NutanixVariableName,
		VariableName,
	)
}

func newNutanixPrismCentralEndpoint(
	client ctrlclient.Client,
	variableName string,
	variableFieldPath ...string,
) *nutanixPrismCentralEndpoint {
	return &nutanixPrismCentralEndpoint{
		cl:                client,
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
	_ mutation.ClusterGetter,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	prismCentralEndpointVar, err := variables.Get[v1alpha1.NutanixPrismCentralEndpointSpec](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("Nutanix PrismCentralEndpoint variable not defined")
			return nil
		}
		return err
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
			var port uint16
			address, port, err = prismCentralEndpointVar.ParseURL()
			if err != nil {
				return err
			}

			pcCredSecretName := prismCentralEndpointVar.Credentials.SecretRef.Name
			if prismCentralEndpointVar.Credentials.SecretRef.Name == "" {
				pcCredSecretName = NutanixPCCredentialsSecretName(clusterKey.Name)
				pcCredRequestObj := NutanixPCCreentialsRequest(
					clusterKey.Name,
					clusterKey.Namespace,
					pcCredSecretName,
				)
				handlersutils.CreateNutanixCredentialsRequest(ctx, h.cl, pcCredRequestObj)
			}
			prismCentral := &credentials.NutanixPrismEndpoint{
				Address:  address,
				Port:     int32(port),
				Insecure: prismCentralEndpointVar.Insecure,
				CredentialRef: &credentials.NutanixCredentialReference{
					Kind: credentials.SecretKind,
					Name: pcCredSecretName,
					// Assume the secret is in the same namespace as Cluster
					Namespace: clusterKey.Namespace,
				},
			}
			additionalTrustBundle := prismCentralEndpointVar.AdditionalTrustBundle
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
