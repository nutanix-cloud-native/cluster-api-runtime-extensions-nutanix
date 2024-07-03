// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package wait

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CheckFailedError is used to determine whether the wait failed because wraps an error returned by a failed check.
type CheckFailedError struct {
	cause error
}

func (e *CheckFailedError) Error() string {
	return fmt.Sprintf("check failed: %s", e.cause)
}

func (e *CheckFailedError) Is(target error) bool {
	_, ok := target.(*CheckFailedError)
	return ok
}

func (e *CheckFailedError) Unwrap() error {
	return e.cause
}

type ForObjectInput[T client.Object] struct {
	Reader   client.Reader
	Target   T
	Check    func(ctx context.Context, obj T) (bool, error)
	Interval time.Duration
	Timeout  time.Duration
}

func ForObject[T client.Object](
	ctx context.Context,
	input ForObjectInput[T],
) error {
	key := client.ObjectKeyFromObject(input.Target)

	// TODO Rename this to be generic, and store ErrConditionOutOfDate or ErrStatusOutOfDate.
	var getErr error
	waitErr := wait.PollUntilContextTimeout(
		ctx,
		input.Interval,
		input.Timeout,
		true,
		func(checkCtx context.Context) (bool, error) {
			if getErr = input.Reader.Get(checkCtx, key, input.Target); getErr != nil {
				if apierrors.IsNotFound(getErr) {
					return false, nil
				}
				return false, getErr
			}

			if ok, err := input.Check(checkCtx, input.Target); err != nil {
				return false, &CheckFailedError{cause: err}
			} else {
				// Retry if check fails.
				return ok, nil
			}
		})

	if wait.Interrupted(waitErr) {
		if getErr != nil {
			return fmt.Errorf("%w; last get error: %w", waitErr, getErr)
		}
		return fmt.Errorf("%w: check never passed", waitErr)
	}
	// waitErr is a CheckFailedError
	return waitErr
}
