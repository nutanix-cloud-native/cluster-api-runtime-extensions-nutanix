// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package namespacesync

import (
	"context"
	"errors"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/storage/names"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/internal/test/builder"
)

const (
	sourceNamespace = "source-ns"
	targetNamespace = "target-ns"
)

var errFakeCreate = errors.New("fake create error")

type mockWriter struct {
	client.Writer
	createErrOnKind string
	createdObjects  []client.Object
}

func (m *mockWriter) Create(
	ctx context.Context,
	obj client.Object,
	opts ...client.CreateOption,
) error {
	if m.createErrOnKind != "" && obj.GetObjectKind().GroupVersionKind().Kind == m.createErrOnKind {
		return errFakeCreate
	}
	m.createdObjects = append(m.createdObjects, obj)
	// Fake setting of UID to simulate a real API server create.
	obj.SetUID("fake-uid")
	return nil
}

func TestCopyClusterClassAndTemplates(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	testCases := []struct {
		name            string
		createErrOnKind string
		expectErr       error
		expectNumCopies int
	}{
		{
			name:            "should succeed if all objects are created",
			expectNumCopies: 6, // 1 ClusterClass + 5 templates
		},
		{
			name:            "should fail if creating a template fails",
			createErrOnKind: "GenericInfrastructureClusterTemplate",
			expectErr:       errFakeCreate,
			expectNumCopies: 0, // The first template create will fail.
		},
		{
			name:            "should fail if creating the clusterclass fails",
			createErrOnKind: "ClusterClass",
			expectErr:       errFakeCreate,
			expectNumCopies: 5, // All 5 templates are created before ClusterClass.
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sourceClusterClass, sourceTemplates := newTestClusterClassAndTemplates(
				sourceNamespace,
				names.SimpleNameGenerator.GenerateName("test-cc-"),
			)
			initObjs := []runtime.Object{sourceClusterClass}
			for _, template := range sourceTemplates {
				initObjs = append(initObjs, template)
			}

			fakeReader := fake.NewClientBuilder().WithRuntimeObjects(initObjs...).Build()
			mockWriter := &mockWriter{
				createErrOnKind: tc.createErrOnKind,
			}

			err := copyClusterClassAndTemplates(
				ctx,
				mockWriter,
				fakeReader,
				sourceClusterClass,
				targetNamespace,
			)

			if tc.expectErr != nil {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(ContainSubstring(tc.expectErr.Error())))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			g.Expect(len(mockWriter.createdObjects)).To(Equal(tc.expectNumCopies))

			for _, obj := range mockWriter.createdObjects {
				g.Expect(obj.GetNamespace()).To(Equal(targetNamespace))
				g.Expect(obj.GetOwnerReferences()).To(BeEmpty())
				g.Expect(obj.GetUID()).ToNot(BeEmpty())
				g.Expect(obj.GetResourceVersion()).To(BeEmpty())
			}
		})
	}
}

// newTestClusterClassAndTemplates is a helper to generate a valid ClusterClass with all its referenced templates.
func newTestClusterClassAndTemplates(
	namespace,
	prefix string,
) (*clusterv1.ClusterClass, []client.Object) {
	bootstrapTemplate := builder.BootstrapTemplate(namespace, prefix).Build()
	infraMachineTemplateControlPlane := builder.InfrastructureMachineTemplate(
		namespace,
		fmt.Sprintf("%s-control-plane", prefix),
	).Build()
	infraMachineTemplateWorker := builder.InfrastructureMachineTemplate(
		namespace,
		fmt.Sprintf("%s-worker", prefix),
	).Build()
	controlPlaneTemplate := builder.ControlPlaneTemplate(namespace, prefix).Build()
	infraClusterTemplate := builder.InfrastructureClusterTemplate(namespace, prefix).Build()
	machineDeploymentClass := builder.MachineDeploymentClass(fmt.Sprintf("%s-worker", prefix)).
		WithBootstrapTemplate(bootstrapTemplate).
		WithInfrastructureTemplate(infraMachineTemplateWorker).
		Build()
	clusterClass := builder.ClusterClass(namespace, prefix).
		WithInfrastructureClusterTemplate(infraClusterTemplate).
		WithControlPlaneTemplate(controlPlaneTemplate).
		WithControlPlaneInfrastructureMachineTemplate(infraMachineTemplateControlPlane).
		WithWorkerMachineDeploymentClasses(*machineDeploymentClass).
		Build()

	templates := []client.Object{
		bootstrapTemplate,
		infraMachineTemplateWorker,
		infraMachineTemplateControlPlane,
		controlPlaneTemplate,
		infraClusterTemplate,
	}

	return clusterClass, templates
}
