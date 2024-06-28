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
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestWait(t *testing.T) {
	// We use the corev1.Namespace concrete type for the test, because we want to
	// verify behavior for a concrete type, and because the Wait function is
	// generic, and will behave identically for all concrete types.
	type args struct {
		input ForObjectInput[*corev1.Namespace]
	}
	tests := []struct {
		name     string
		args     args
		errCheck func(error) bool
	}{
		{
			name: "time out while get fails; report get error",
			args: args{
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
			},
			errCheck: func(err error) bool {
				return wait.Interrupted(err) &&
					apierrors.IsNotFound(err)
			},
		},
		{
			name: "time out while check returns false; no check error to report",
			args: args{
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
			},
			errCheck: wait.Interrupted,
		},
		{
			name: "return immediately when check returns an error; report the error",
			args: args{
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
				context.Background(),
				tt.args.input,
			)
			if !tt.errCheck(err) {
				t.Errorf("error did not pass check: %s", err)
			}
		})
	}
}
