//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"time"

	prismcommonapi "github.com/nutanix/ntnx-api-golang-clients/prism-go-client/v4/models/common/v1/config"
	prismapi "github.com/nutanix/ntnx-api-golang-clients/prism-go-client/v4/models/prism/v4/config"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	v4Converged "github.com/nutanix-cloud-native/prism-go-client/converged/v4"
)

const (
	prismEndpointVariableName = "NUTANIX_ENDPOINT"
	prismPortVariableName     = "NUTANIX_PORT"
	prismUsernameVariableName = "NUTANIX_USER"
	prismPasswordVariableName = "NUTANIX_PASSWORD"
)

func CredentialsFromCAPIE2EConfig(e2eConfig *clusterctl.E2EConfig) *prismgoclient.Credentials {
	return &prismgoclient.Credentials{
		Endpoint: e2eConfig.MustGetVariable(prismEndpointVariableName),
		Port:     e2eConfig.MustGetVariable(prismPortVariableName),
		Username: e2eConfig.MustGetVariable(prismUsernameVariableName),
		Password: e2eConfig.MustGetVariable(prismPasswordVariableName),
		Insecure: false,
	}
}

// NewConvergedV4Client creates a converged V4 client from credentials.
func NewConvergedV4Client(credentials *prismgoclient.Credentials) (*v4Converged.Client, error) {
	client, err := v4Converged.NewClient(*credentials)
	if err != nil {
		return nil, fmt.Errorf("failed to create converged V4 client: %w", err)
	}

	return client, nil
}

func WaitForTaskCompletion(
	ctx context.Context,
	taskID string,
	convergedClient *v4Converged.Client,
) ([]prismcommonapi.KVPair, error) {
	var data []prismcommonapi.KVPair

	if err := wait.PollUntilContextCancel(
		ctx,
		1*time.Second,
		false,
		func(ctx context.Context) (done bool, err error) {
			task, err := convergedClient.Tasks.Get(ctx, taskID)
			if err != nil {
				return false, fmt.Errorf("failed to get task %s: %w", taskID, err)
			}

			if ptr.Deref(task.Status, prismapi.TASKSTATUS_UNKNOWN) != prismapi.TASKSTATUS_SUCCEEDED {
				return false, nil
			}

			data = task.CompletionDetails

			return true, nil
		},
	); err != nil {
		return nil, fmt.Errorf("failed to wait for task %s to complete: %w", taskID, err)
	}

	return data, nil
}
