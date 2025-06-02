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
	prismclientv4 "github.com/nutanix-cloud-native/prism-go-client/v4"
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

func NewV4Client(credentials *prismgoclient.Credentials) (*prismclientv4.Client, error) {
	v4Client, err := prismclientv4.NewV4Client(*credentials)
	if err != nil {
		return nil, fmt.Errorf("failed to create Nutanix V4 API client: %w", err)
	}

	return v4Client, nil
}

func WaitForTaskCompletion(
	ctx context.Context,
	taskID string,
	v4Client *prismclientv4.Client,
) ([]prismcommonapi.KVPair, error) {
	var data []prismcommonapi.KVPair

	if err := wait.PollUntilContextCancel(
		ctx,
		100*time.Millisecond,
		true,
		func(ctx context.Context) (done bool, err error) {
			task, err := v4Client.TasksApiInstance.GetTaskById(ptr.To(taskID))
			if err != nil {
				return false, fmt.Errorf("failed to get task %s: %w", taskID, err)
			}

			taskData, ok := task.GetData().(prismapi.Task)
			if !ok {
				return false, fmt.Errorf("unexpected task data type %[1]T: %+[1]v", task.GetData())
			}

			if ptr.Deref(taskData.Status, prismapi.TASKSTATUS_UNKNOWN) != prismapi.TASKSTATUS_SUCCEEDED {
				return false, nil
			}

			data = taskData.CompletionDetails

			return true, nil
		},
	); err != nil {
		return nil, fmt.Errorf("failed to wait for task %s to complete: %w", taskID, err)
	}

	return data, nil
}
