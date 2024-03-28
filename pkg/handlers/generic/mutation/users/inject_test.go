// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package users

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/utils/ptr"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func Test_generateBootstrapUser(t *testing.T) {
	type args struct {
		userFromVariable v1alpha1.User
	}
	tests := []struct {
		name string
		args args
		want bootstrapv1.User
	}{
		{
			name: "if user sets hashed password, enable password auth and set passwd",
			args: args{
				userFromVariable: v1alpha1.User{
					Name:           "example",
					HashedPassword: "example",
				},
			},
			want: bootstrapv1.User{
				Name:         "example",
				Passwd:       ptr.To("example"),
				LockPassword: ptr.To(false),
			},
		},
		{
			name: "if user does not set hashed password, disable password auth and do not set passwd",
			args: args{
				userFromVariable: v1alpha1.User{
					Name: "example",
				},
			},
			want: bootstrapv1.User{
				Name:         "example",
				Passwd:       nil,
				LockPassword: ptr.To(true),
			},
		},
		{
			name: "if user sets empty hashed password, disable password auth and do not set passwd",
			args: args{
				userFromVariable: v1alpha1.User{
					Name:           "example",
					HashedPassword: "",
				},
			},
			want: bootstrapv1.User{
				Name:         "example",
				Passwd:       nil,
				LockPassword: ptr.To(true),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateBootstrapUser(tt.args.userFromVariable)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("generateBootstrapUser() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
