// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package metallb

import (
	"context"
	"fmt"

	"github.com/samber/lo"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	metallbv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/go.universe.tf/metallb/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	k8sclient "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
)

type ConfigurationInput struct {
	Name          string
	Namespace     string
	AddressRanges []v1alpha1.AddressRange
}

func HelmChartProxyName(clusterUUID string) string {
	return fmt.Sprintf("%s-%s", DefaultHelmReleaseName, clusterUUID)
}

func ConfigurationObjects(input *ConfigurationInput) ([]ctrlclient.Object, error) {
	if len(input.AddressRanges) == 0 {
		return nil, fmt.Errorf("must define one or more AddressRanges")
	}

	ipAddressPool := &metallbv1.IPAddressPool{
		TypeMeta: metav1.TypeMeta{
			Kind:       "IPAddressPool",
			APIVersion: metallbv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      input.Name,
			Namespace: input.Namespace,
		},
		Spec: metallbv1.IPAddressPoolSpec{
			Addresses: lo.Map(input.AddressRanges, func(ar v1alpha1.AddressRange, _ int) string {
				return fmt.Sprintf("%s-%s", ar.Start, ar.End)
			}),
		},
	}

	l2Advertisement := &metallbv1.L2Advertisement{
		TypeMeta: metav1.TypeMeta{
			Kind:       "L2Advertisement",
			APIVersion: metallbv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      input.Name,
			Namespace: input.Namespace,
		},
		Spec: metallbv1.L2AdvertisementSpec{
			IPAddressPools: []string{ipAddressPool.GetName()},
		},
	}

	return []ctrlclient.Object{
		ipAddressPool,
		l2Advertisement,
	}, nil
}

func ApplyConfiguration(
	ctx context.Context,
	remoteClient ctrlclient.Client,
	cfg *v1alpha1.ServiceLoadBalancerConfiguration,
) error {
	if cfg == nil {
		return nil
	}

	configInput := &ConfigurationInput{
		Name:          DefaultHelmReleaseName,
		Namespace:     DefaultHelmReleaseNamespace,
		AddressRanges: cfg.AddressRanges,
	}
	cos, err := ConfigurationObjects(configInput)
	if err != nil {
		return fmt.Errorf("failed to generate MetalLB configuration: %w", err)
	}

	for _, o := range cos {
		err := k8sclient.ServerSideApply(
			ctx,
			remoteClient,
			o,
			&ctrlclient.PatchOptions{
				Raw: &metav1.PatchOptions{
					FieldValidation: metav1.FieldValidationStrict,
				},
			},
		)
		if err == nil {
			continue
		}

		if apierrors.IsConflict(err) {
			switch o.(type) {
			case *metallbv1.IPAddressPool:
				err = fmt.Errorf(
					"%w. This resource has been modified in the workload cluster: it must contain exactly the addresses listed in the Cluster configuration", //nolint:lll // User-facing conflict message.
					err,
				)
			case *metallbv1.L2Advertisement:
				err = fmt.Errorf(
					"%w. This resource has been modified in the workload cluster, it must only contain the %q IP Address Pool", //nolint:lll // User-facing conflict message.
					err,
					configInput.Name,
				)
			}

			return fmt.Errorf(
				"failed to apply MetalLB configuration %s %s: %w",
				o.GetObjectKind().GroupVersionKind().Kind,
				ctrlclient.ObjectKeyFromObject(o),
				err,
			)
		}

		return fmt.Errorf(
			"failed to apply MetalLB configuration %s %s: %w",
			o.GetObjectKind().GroupVersionKind().Kind,
			ctrlclient.ObjectKeyFromObject(o),
			err,
		)
	}

	return nil
}
