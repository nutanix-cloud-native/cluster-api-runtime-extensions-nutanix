// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package metallb

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func Test_isRetriableConfigurationApplyError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil",
			err:  nil,
			want: false,
		},
		{
			name: "internal error from webhook",
			err: apierrors.NewInternalError(
				fmt.Errorf("failed calling webhook %q", "ipaddresspoolvalidationwebhook.metallb.io"),
			),
			want: true,
		},
		{
			name: "not found",
			err:  apierrors.NewNotFound(schema.GroupResource{Group: "metallb.io", Resource: "ipaddresspools"}, "metallb"),
			want: true,
		},
		{
			name: "timeout",
			err:  apierrors.NewTimeoutError("request timed out", 1),
			want: true,
		},
		{
			name: "deadline exceeded",
			err:  context.DeadlineExceeded,
			want: true,
		},
		{
			name: "wrapped deadline exceeded",
			err:  fmt.Errorf("server-side apply failed: %w", context.DeadlineExceeded),
			want: true,
		},
		{
			name: "context canceled",
			err:  context.Canceled,
			want: true,
		},
		{
			name: "no matches for kind",
			err:  errors.New(`no matches for kind "IPAddressPool" in version "metallb.io/v1beta1"`),
			want: true,
		},
		{
			name: "failed calling webhook",
			err:  errors.New(`failed calling webhook "ipaddresspoolvalidationwebhook.metallb.io": context deadline exceeded`),
			want: true,
		},
		{
			name: "conflict",
			err:  apierrors.NewConflict(schema.GroupResource{Group: "metallb.io", Resource: "ipaddresspools"}, "metallb", errors.New("conflict")),
			want: false,
		},
		{
			name: "permanent other error",
			err:  errors.New("admission webhook denied the request"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			g.Expect(isRetriableConfigurationApplyError(tt.err)).To(gomega.Equal(tt.want))
		})
	}
}

func Test_configurationConflictError(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)

	objects, err := ConfigurationObjects(&ConfigurationInput{
		Name:      DefaultHelmReleaseName,
		Namespace: DefaultHelmReleaseNamespace,
		AddressRanges: []v1alpha1.AddressRange{
			{Start: "10.0.0.1", End: "10.0.0.10"},
		},
	})
	g.Expect(err).ToNot(gomega.HaveOccurred())

	conflict := apierrors.NewConflict(
		schema.GroupResource{Group: "metallb.io", Resource: "ipaddresspools"},
		"metallb",
		errors.New("conflict"),
	)

	poolErr := configurationConflictError(objects[0], conflict, DefaultHelmReleaseName)
	g.Expect(poolErr.Error()).To(gomega.ContainSubstring("exactly the addresses listed"))

	advErr := configurationConflictError(objects[1], conflict, DefaultHelmReleaseName)
	g.Expect(advErr.Error()).To(gomega.ContainSubstring(DefaultHelmReleaseName))
	g.Expect(advErr.Error()).To(gomega.ContainSubstring("IP Address Pool"))
}

type patchFuncClient struct {
	ctrlclient.Client
	patch func(ctx context.Context, obj ctrlclient.Object, patch ctrlclient.Patch, opts ...ctrlclient.PatchOption) error
}

func (c *patchFuncClient) Patch(
	ctx context.Context,
	obj ctrlclient.Object,
	patch ctrlclient.Patch,
	opts ...ctrlclient.PatchOption,
) error {
	return c.patch(ctx, obj, patch, opts...)
}

func Test_applyConfiguration_failFastOnHang(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)

	objects, err := ConfigurationObjects(&ConfigurationInput{
		Name:      DefaultHelmReleaseName,
		Namespace: DefaultHelmReleaseNamespace,
		AddressRanges: []v1alpha1.AddressRange{
			{Start: "10.0.0.1", End: "10.0.0.10"},
		},
	})
	g.Expect(err).ToNot(gomega.HaveOccurred())

	remoteClient := &patchFuncClient{
		Client: fake.NewClientBuilder().WithScheme(fake.NewClientBuilder().Build().Scheme()).Build(),
		patch: func(ctx context.Context, _ ctrlclient.Object, _ ctrlclient.Patch, _ ...ctrlclient.PatchOption) error {
			<-ctx.Done()
			return fmt.Errorf("server-side apply failed: %w", ctx.Err())
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	err = applyConfiguration(ctx, remoteClient, objects, DefaultHelmReleaseName)
	elapsed := time.Since(start)

	g.Expect(err).To(gomega.HaveOccurred())
	g.Expect(err.Error()).To(gomega.ContainSubstring("failed to apply MetalLB configuration"))
	g.Expect(elapsed).To(gomega.BeNumerically("<", 6*time.Second))
	g.Expect(ctx.Err()).ToNot(gomega.HaveOccurred())
}

func Test_applyConfiguration_conflictFailsImmediately(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)

	objects, err := ConfigurationObjects(&ConfigurationInput{
		Name:      DefaultHelmReleaseName,
		Namespace: DefaultHelmReleaseNamespace,
		AddressRanges: []v1alpha1.AddressRange{
			{Start: "10.0.0.1", End: "10.0.0.10"},
		},
	})
	g.Expect(err).ToNot(gomega.HaveOccurred())

	remoteClient := &patchFuncClient{
		Client: fake.NewClientBuilder().Build(),
		patch: func(_ context.Context, _ ctrlclient.Object, _ ctrlclient.Patch, _ ...ctrlclient.PatchOption) error {
			return apierrors.NewConflict(
				schema.GroupResource{Group: "metallb.io", Resource: "ipaddresspools"},
				"metallb",
				errors.New("conflict"),
			)
		},
	}

	start := time.Now()
	err = applyConfiguration(context.Background(), remoteClient, objects, DefaultHelmReleaseName)
	g.Expect(err).To(gomega.HaveOccurred())
	g.Expect(err.Error()).To(gomega.ContainSubstring("exactly the addresses listed"))
	g.Expect(time.Since(start)).To(gomega.BeNumerically("<", time.Second))
}
