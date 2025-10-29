// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package namespacesync

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/internal/test/builder"
)

func TestWalkReferences(t *testing.T) {
	tests := []struct {
		name         string
		clusterClass *clusterv1.ClusterClass
	}{
		{
			name:         "nil ClusterClass should return nil without calling callback",
			clusterClass: nil,
		},
		{
			name:         "empty ClusterClass with no template references",
			clusterClass: builder.ClusterClass("default", "test-cc").Build(),
		},
		{
			name: "ClusterClass with Infrastructure cluster template reference only",
			clusterClass: builder.ClusterClass("default", "test-cc").
				WithInfrastructureClusterTemplate(
					builder.InfrastructureClusterTemplate("default", "infra-template").Build(),
				).Build(),
		},
		{
			name: "ClusterClass with ControlPlane template reference only",
			clusterClass: builder.ClusterClass("default", "test-cc").
				WithControlPlaneTemplate(
					builder.ControlPlaneTemplate("default", "cp-template").Build(),
				).Build(),
		},
		{
			name: "ClusterClass with MachineInfrastructure template reference",
			clusterClass: builder.ClusterClass("default", "test-cc").
				WithControlPlaneInfrastructureMachineTemplate(
					builder.InfrastructureMachineTemplate("default", "cp-machine-template").Build(),
				).Build(),
		},
		{
			name: "ClusterClass with MachineDeployments template references",
			clusterClass: builder.ClusterClass("default", "test-cc").
				WithWorkerMachineDeploymentClasses(
					*builder.MachineDeploymentClass("worker-1").
						WithInfrastructureTemplate(
							builder.InfrastructureMachineTemplate("default", "worker-infra-template").Build(),
						).
						WithBootstrapTemplate(
							builder.BootstrapTemplate("default", "worker-bootstrap-template").Build(),
						).Build(),
				).Build(),
		},
		{
			name: "ClusterClass with MachineDeployments having nil Infrastructure template reference",
			clusterClass: builder.ClusterClass("default", "test-cc").
				WithWorkerMachineDeploymentClasses(
					*builder.MachineDeploymentClass("worker-1").
						WithBootstrapTemplate(
							builder.BootstrapTemplate("default", "worker-bootstrap-template").Build(),
						).Build(),
				).Build(),
		},
		{
			name: "ClusterClass with MachineDeployments having nil Bootstrap template reference",
			clusterClass: builder.ClusterClass("default", "test-cc").
				WithWorkerMachineDeploymentClasses(
					*builder.MachineDeploymentClass("worker-1").
						WithInfrastructureTemplate(
							builder.InfrastructureMachineTemplate("default", "worker-infra-template").Build(),
						).Build(),
				).Build(),
		},
		{
			name: "callback function returns error",
			clusterClass: builder.ClusterClass("default", "test-cc").
				WithInfrastructureClusterTemplate(
					builder.InfrastructureClusterTemplate("default", "infra-template").Build(),
				).Build(),
		},
		{
			name: "complete ClusterClass with all template references",
			clusterClass: builder.ClusterClass("default", "test-cc").
				WithInfrastructureClusterTemplate(
					builder.InfrastructureClusterTemplate("default", "infra-template").Build(),
				).
				WithControlPlaneTemplate(
					builder.ControlPlaneTemplate("default", "cp-template").Build(),
				).
				WithControlPlaneInfrastructureMachineTemplate(
					builder.InfrastructureMachineTemplate("default", "cp-machine-template").Build(),
				).
				WithWorkerMachineDeploymentClasses(
					*builder.MachineDeploymentClass("worker-1").
						WithInfrastructureTemplate(
							builder.InfrastructureMachineTemplate("default", "worker-infra-template").Build(),
						).
						WithBootstrapTemplate(
							builder.BootstrapTemplate("default", "worker-bootstrap-template").Build(),
						).Build(),
					*builder.MachineDeploymentClass("worker-2").
						WithInfrastructureTemplate(
							builder.InfrastructureMachineTemplate("default", "worker2-infra-template").Build(),
						).
						WithBootstrapTemplate(
							builder.BootstrapTemplate("default", "worker2-bootstrap-template").Build(),
						).Build(),
				).Build(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			ctx := context.Background()
			var capturedRefs []*corev1.ObjectReference

			callback := func(ctx context.Context, ref *corev1.ObjectReference) error {
				capturedRefs = append(capturedRefs, ref)
				return nil
			}

			err := walkReferences(ctx, tt.clusterClass, callback)

			g.Expect(err).ToNot(HaveOccurred())

			// Verify that captured references match expected ones
			if tt.clusterClass != nil {
				expectedRefs := collectExpectedRefs(tt.clusterClass)
				g.Expect(capturedRefs).
					To(HaveLen(len(expectedRefs)), "Expected %d references, got %d", len(expectedRefs), len(capturedRefs))

				for i, expectedRef := range expectedRefs {
					if expectedRef != nil {
						g.Expect(capturedRefs[i]).To(Equal(expectedRef), "Reference doesn't match")
					}
				}
			}
		})
	}
}

func collectExpectedRefs(cc *clusterv1.ClusterClass) []*corev1.ObjectReference {
	var refs []*corev1.ObjectReference

	if cc.Spec.Infrastructure.Ref != nil {
		refs = append(refs, cc.Spec.Infrastructure.Ref)
	}

	if cc.Spec.ControlPlane.Ref != nil {
		refs = append(refs, cc.Spec.ControlPlane.Ref)
	}

	if cpInfra := cc.Spec.ControlPlane.MachineInfrastructure; cpInfra != nil && cpInfra.Ref != nil {
		refs = append(refs, cpInfra.Ref)
	}

	for _, md := range cc.Spec.Workers.MachineDeployments {
		if md.Template.Infrastructure.Ref != nil {
			refs = append(refs, md.Template.Infrastructure.Ref)
		}
		if md.Template.Bootstrap.Ref != nil {
			refs = append(refs, md.Template.Bootstrap.Ref)
		}
	}

	return refs
}
