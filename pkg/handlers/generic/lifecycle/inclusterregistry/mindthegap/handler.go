// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mindthegap

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

type Config struct {
	*options.GlobalOptions

	defaultValuesTemplateConfigMapName string
}

type Mindthegap struct {
	client ctrlclient.Client
	config *Config
}

func New(
	c ctrlclient.Client,
	cfg *Config,
) *Mindthegap {
	return &Mindthegap{
		client: c,
		config: cfg,
	}
}

func (n *Mindthegap) Apply(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	log logr.Logger,
) error {
	log.Info("Applying mindthegap in-cluster registry installation")

	remoteClient, err := remote.NewClusterClient(
		ctx,
		"",
		n.client,
		ctrlclient.ObjectKeyFromObject(cluster),
	)
	if err != nil {
		return fmt.Errorf("error creating remote cluster client: %w", err)
	}

	log.Info(
		fmt.Sprintf("Applying mindthegap in-cluster registry to cluster %s",
			ctrlclient.ObjectKeyFromObject(cluster),
		),
	)

	configurationInput := &ConfigurationInput{
		Cluster: cluster,
	}
	cos, err := RemoteClusterObjects(configurationInput)
	if err != nil {
		return fmt.Errorf("failed to generate mindthegap configuration: %w", err)
	}
	err = applyObjs(ctx, remoteClient, cos)
	if err != nil {
		return fmt.Errorf("failed to apply mindthegap configuration: %w", err)
	}

	log.Info(
		fmt.Sprintf("Applying utility to seed image bundles for cluster %s",
			ctrlclient.ObjectKeyFromObject(cluster),
		),
	)

	cos, err = ManagementClusterObjects(configurationInput)
	if err != nil {
		return fmt.Errorf("failed to generate utility to seed image configuration: %w", err)
	}
	err = applyObjs(ctx, n.client, cos)
	if err != nil {
		return fmt.Errorf("failed to apply utility to seed image configuration: %w", err)
	}
	err = addClusterOwnerReferenceToObjs(ctx, n.client, cluster, cos)
	if err != nil {
		return fmt.Errorf("failed to add owner to seed image configuration: %w", err)
	}

	return nil
}

func applyObjs(ctx context.Context, cl ctrlclient.Client, objs []unstructured.Unstructured) error {
	for i := range objs {
		o := &objs[i]
		err := client.ServerSideApply(
			ctx,
			cl,
			o,
			&ctrlclient.PatchOptions{
				Raw: &metav1.PatchOptions{
					FieldValidation: metav1.FieldValidationStrict,
				},
			},
		)
		if err != nil {
			return fmt.Errorf(
				"failed to apply object %s %s: %w",
				o.GetKind(),
				ctrlclient.ObjectKeyFromObject(o),
				err,
			)
		}
	}

	return nil
}

func addClusterOwnerReferenceToObjs(
	ctx context.Context,
	cl ctrlclient.Client,
	cluster *clusterv1.Cluster,
	objs []unstructured.Unstructured,
) error {
	for i := range objs {
		o := &objs[i]
		err := handlersutils.EnsureClusterOwnerReferenceForObject(
			ctx,
			cl,
			corev1.TypedLocalObjectReference{
				APIGroup: ptr.To(o.GetAPIVersion()),
				Kind:     o.GetKind(),
				Name:     o.GetName(),
			},
			cluster,
		)
		if err != nil {
			return fmt.Errorf("failed to set owner reference for object %s %s: %w",
				o.GetKind(),
				ctrlclient.ObjectKeyFromObject(o),
				err,
			)
		}
	}

	return nil
}
