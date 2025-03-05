// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package wait

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var errBrokenReader = errors.New("broken")

type brokenReader struct{}

func (r *brokenReader) Get(
	ctx context.Context,
	key client.ObjectKey,
	obj client.Object,
	opts ...client.GetOption,
) error {
	return errBrokenReader
}

func (r *brokenReader) List(
	ctx context.Context,
	list client.ObjectList,
	opts ...client.ListOption,
) error {
	return errBrokenReader
}

var _ client.Reader = &brokenReader{}

func TestWait(t *testing.T) {
	tests := []struct {
		name string
		// We use the corev1.Namespace concrete type for the test, because we want to
		// verify behavior for a concrete type, and because the Wait function is
		// generic, and will behave identically for all concrete types.
		input    ForObjectInput[*corev1.Namespace]
		errCheck func(error) bool
	}{
		{
			name: "time out while get does not find object; report get error",
			input: ForObjectInput[*corev1.Namespace]{
				Reader: fake.NewFakeClient(),
				Check: func(_ context.Context, _ *corev1.Namespace) (bool, error) {
					return true, nil
				},
				Interval: time.Nanosecond,
				Timeout:  time.Millisecond,
				Target: &corev1.Namespace{
					TypeMeta: v1.TypeMeta{
						Kind:       "Namespace",
						APIVersion: "v1",
					},
					ObjectMeta: v1.ObjectMeta{
						Name: "example",
					},
				},
			},
			errCheck: func(err error) bool {
				return wait.Interrupted(err) &&
					apierrors.IsNotFound(err)
			},
		},
		{
			name: "return immediately when get fails; report get error",
			input: ForObjectInput[*corev1.Namespace]{
				Reader: &brokenReader{},
				Check: func(_ context.Context, _ *corev1.Namespace) (bool, error) {
					return true, nil
				},
				Interval: time.Nanosecond,
				Timeout:  time.Millisecond,
				Target: &corev1.Namespace{
					TypeMeta: v1.TypeMeta{
						Kind:       "Namespace",
						APIVersion: "v1",
					},
					ObjectMeta: v1.ObjectMeta{
						Name: "example",
					},
				},
			},
			errCheck: func(err error) bool {
				return !wait.Interrupted(err) &&
					!apierrors.IsNotFound(err) &&
					errors.Is(err, errBrokenReader)
			},
		},
		{
			name: "time out while check returns false; no check error to report",
			input: ForObjectInput[*corev1.Namespace]{
				Reader: fake.NewFakeClient(
					&corev1.Namespace{
						TypeMeta: v1.TypeMeta{
							Kind:       "Namespace",
							APIVersion: "v1",
						},
						ObjectMeta: v1.ObjectMeta{
							Name: "example",
						},
					},
				),
				Check: func(_ context.Context, _ *corev1.Namespace) (bool, error) {
					return false, nil
				},
				Interval: time.Nanosecond,
				Timeout:  time.Millisecond,
				Target: &corev1.Namespace{
					TypeMeta: v1.TypeMeta{
						Kind:       "Namespace",
						APIVersion: "v1",
					},
					ObjectMeta: v1.ObjectMeta{
						Name: "example",
					},
				},
			},
			errCheck: wait.Interrupted,
		},
		{
			name: "return immediately when check returns an error; report the error",
			input: ForObjectInput[*corev1.Namespace]{
				Reader: fake.NewFakeClient(
					&corev1.Namespace{
						TypeMeta: v1.TypeMeta{
							Kind:       "Namespace",
							APIVersion: "v1",
						},
						ObjectMeta: v1.ObjectMeta{
							Name: "example",
						},
					},
				),
				Check: func(_ context.Context, _ *corev1.Namespace) (bool, error) {
					return false, fmt.Errorf("condition failed")
				},
				Interval: time.Nanosecond,
				Timeout:  time.Millisecond, Target: &corev1.Namespace{
					TypeMeta: v1.TypeMeta{
						Kind:       "Namespace",
						APIVersion: "v1",
					},
					ObjectMeta: v1.ObjectMeta{
						Name: "example",
					},
				},
			},
			errCheck: func(err error) bool {
				return errors.Is(err, &CheckFailedError{}) &&
					!wait.Interrupted(err)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ForObject(
				t.Context(),
				tt.input,
			)
			if !tt.errCheck(err) {
				t.Errorf("error did not pass check: %s", err)
			}
		})
	}
}
