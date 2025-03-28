package utils

import (
	"context"
	"fmt"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	k8sClient "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateNutanixCredentialsRequest(
	ctx context.Context,
	cl ctrlclient.Client,
	credentialsRequest *carenv1.CredentialsRequest,
) error {
	if err := k8sClient.ServerSideApply(ctx, cl, credentialsRequest, client.ForceOwnership); err != nil {
		return fmt.Errorf(
			"error creating Nutanix Prism Central Credentials Request: %w",
			err,
		)
	}
	return nil
}
