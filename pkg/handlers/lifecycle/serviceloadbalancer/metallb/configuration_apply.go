// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package metallb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	metallbv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/go.universe.tf/metallb/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
)

const (
	// Keep the MetalLB CR apply well under CAPI's 10s AfterControlPlaneInitialized HTTP
	// deadline so the hook can return Failure and be retried.
	configurationApplyAttemptTimeout = 1500 * time.Millisecond
	configurationApplyTimeout        = 3 * time.Second
	configurationApplyInterval       = 500 * time.Millisecond
	hookResponseHeadroom             = time.Second
)

func applyConfiguration(
	ctx context.Context,
	remoteClient ctrlclient.Client,
	objects []ctrlclient.Object,
	poolName string,
) error {
	applyTimeout, err := configurationApplyBudget(ctx)
	if err != nil {
		return err
	}

	var applyErr error
	waitErr := kwait.PollUntilContextTimeout(
		ctx,
		configurationApplyInterval,
		applyTimeout,
		true,
		func(ctx context.Context) (bool, error) {
			attemptCtx, cancel := context.WithTimeout(ctx, configurationApplyAttemptTimeout)
			defer cancel()

			for _, o := range objects {
				err := client.ServerSideApply(
					attemptCtx,
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
					applyErr = fmt.Errorf(
						"failed to apply MetalLB configuration %s %s: %w",
						o.GetObjectKind().GroupVersionKind().Kind,
						ctrlclient.ObjectKeyFromObject(o),
						configurationConflictError(o, err, poolName),
					)
					return false, applyErr
				}

				if isRetriableConfigurationApplyError(err) {
					applyErr = err
					return false, nil
				}

				return false, fmt.Errorf(
					"failed to apply MetalLB configuration %s %s: %w",
					o.GetObjectKind().GroupVersionKind().Kind,
					ctrlclient.ObjectKeyFromObject(o),
					err,
				)
			}

			return true, nil
		},
	)
	if waitErr != nil {
		if applyErr != nil {
			return fmt.Errorf("failed to apply MetalLB configuration: %w: last apply error: %w", waitErr, applyErr)
		}
		return fmt.Errorf("failed to apply MetalLB configuration: %w", waitErr)
	}

	return nil
}

func configurationApplyBudget(ctx context.Context) (time.Duration, error) {
	applyTimeout := configurationApplyTimeout
	deadline, ok := ctx.Deadline()
	if !ok {
		return applyTimeout, nil
	}

	remaining := time.Until(deadline) - hookResponseHeadroom
	if remaining <= 0 {
		return 0, fmt.Errorf("not enough time remaining to apply MetalLB configuration")
	}
	if remaining < applyTimeout {
		return remaining, nil
	}
	return applyTimeout, nil
}

func isRetriableConfigurationApplyError(err error) bool {
	if err == nil {
		return false
	}
	if apierrors.IsInternalError(err) ||
		apierrors.IsNotFound(err) ||
		apierrors.IsTimeout(err) ||
		apierrors.IsServerTimeout(err) ||
		apierrors.IsTooManyRequests(err) ||
		apierrors.IsServiceUnavailable(err) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}

	msg := err.Error()
	return strings.Contains(msg, "failed calling webhook") ||
		strings.Contains(msg, "no matches for kind") ||
		strings.Contains(msg, "no kind is registered") ||
		strings.Contains(msg, "could not find the requested resource")
}

func configurationConflictError(obj ctrlclient.Object, err error, poolName string) error {
	switch obj.(type) {
	case *metallbv1.IPAddressPool:
		return fmt.Errorf(
			"%w. This resource has been modified in the workload cluster: "+
				"it must contain exactly the addresses listed in the Cluster configuration",
			err,
		)
	case *metallbv1.L2Advertisement:
		return fmt.Errorf(
			"%w. This resource has been modified in the workload cluster, "+
				"it must only contain the %q IP Address Pool",
			err,
			poolName,
		)
	default:
		return err
	}
}
