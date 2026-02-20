// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package namespacesync

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/internal/test/builder"
)

func TestReconcileExistingNamespaceWithUpdatedLabels(t *testing.T) {
	g := NewWithT(t)
	timeout := 5 * time.Second

	sourceClusterClassName, _, cleanup, err := createUniqueClusterClassAndTemplates(
		sourceClusterClassNamespace,
	)
	g.Expect(err).ToNot(HaveOccurred())
	defer func() {
		g.Expect(cleanup()).To(Succeed())
	}()

	// Create namespace without label
	targetNamespace, err := env.CreateNamespace(ctx, "target", map[string]string{})
	g.Expect(err).ToNot(HaveOccurred())

	// Label the namespace
	targetNamespace.Labels[targetNamespaceLabelKey] = ""
	err = env.Update(ctx, targetNamespace)
	g.Expect(err).ToNot(HaveOccurred())

	g.Eventually(func() error {
		return verifyClusterClassAndTemplates(
			env.Client,
			sourceClusterClassName,
			targetNamespace.Name,
		)
	},
		timeout,
	).Should(Succeed())
}

func TestReconcileNewNamespaces(t *testing.T) {
	g := NewWithT(t)
	timeout := 5 * time.Second

	sourceClusterClassName, _, cleanup, err := createUniqueClusterClassAndTemplates(
		sourceClusterClassNamespace,
	)
	g.Expect(err).ToNot(HaveOccurred())
	defer func() {
		g.Expect(cleanup()).To(Succeed())
	}()

	targetNamespaces, err := createTargetNamespaces(3)
	g.Expect(err).ToNot(HaveOccurred())

	for _, targetNamespace := range targetNamespaces {
		g.Eventually(func() error {
			return verifyClusterClassAndTemplates(
				env.Client,
				sourceClusterClassName,
				targetNamespace.Name,
			)
		},
			timeout,
		).Should(Succeed())
	}
}

func TestReconcileNewClusterClass(t *testing.T) {
	g := NewWithT(t)
	timeout := 5 * time.Second

	targetNamespaces, err := createTargetNamespaces(3)
	g.Expect(err).ToNot(HaveOccurred())

	testCases := []struct {
		name   string
		create func(namespace string) (string, []client.Object, func() error, error)
	}{
		{
			name:   "cluster class",
			create: createUniqueClusterClassAndTemplates,
		},
		{
			name:   "managed cluster class",
			create: createUniqueManagedClusterClassAndTemplates,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			sourceClusterClassName, _, cleanup, err := tc.create(sourceClusterClassNamespace)
			g.Expect(err).ToNot(HaveOccurred())
			defer func() {
				g.Expect(cleanup()).To(Succeed())
			}()
			for _, targetNamespace := range targetNamespaces {
				g.Eventually(func() error {
					return verifyClusterClassAndTemplates(
						env.Client,
						sourceClusterClassName,
						targetNamespace.Name,
					)
				},
					timeout,
				).Should(Succeed())
			}
		})
	}
}

func TestSourceClusterClassNamespaceEmpty(t *testing.T) {
	g := NewWithT(t)

	_, _, cleanup, err := createUniqueClusterClassAndTemplates(
		sourceClusterClassNamespace,
	)
	g.Expect(err).ToNot(HaveOccurred())
	defer func() {
		g.Expect(cleanup()).To(Succeed())
	}()

	// This test initializes its own reconciler, instead of using the one created
	// in suite_test.go, in order to configure the source namespace.
	r := Reconciler{
		Client:                      env.Client,
		SourceClusterClassNamespace: "",
	}

	ns, err := r.listSourceClusterClasses(ctx)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(ns).To(BeEmpty())
}

func TestReconcileAfterPartialFailureToCopy(t *testing.T) {
	g := NewWithT(t)
	timeout := 5 * time.Second
	prefix := "partial-failure"

	sourceClusterClassName, sourceTemplates, sourceCleanup, err := createClusterClassAndTemplates(
		prefix,
		sourceClusterClassNamespace,
	)
	g.Expect(err).ToNot(HaveOccurred())
	defer func() {
		g.Expect(sourceCleanup()).To(Succeed())
	}()

	// Create a namespace with the same ClusterClass and templates.
	targetNamespace, err := env.CreateNamespace(ctx, "target", map[string]string{})
	g.Expect(err).ToNot(HaveOccurred())
	targetClusterClassName, targetTemplates, _, err := createClusterClassAndTemplates(
		prefix,
		targetNamespace.Name,
	)
	g.Expect(err).ToNot(HaveOccurred())

	// Verify that the target ClusterClass and templates match the source.
	g.Expect(targetClusterClassName).To(Equal(sourceClusterClassName))
	for i := range targetTemplates {
		g.Expect(targetTemplates[i].GetName()).To(Equal(sourceTemplates[i].GetName()))
	}

	// Simulate a partial copy failure by removing the ClusterClass--the last object copied--from the target namespace.
	g.Expect(env.CleanupAndWait(ctx, &clusterv1.ClusterClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetClusterClassName,
			Namespace: targetNamespace.Name,
		},
	})).To(Succeed())

	// Label the namespace so that it is considered a target namespace to trigger the reconcile loop.
	targetNamespace.Labels[targetNamespaceLabelKey] = ""
	err = env.Update(ctx, targetNamespace)
	g.Expect(err).ToNot(HaveOccurred())

	// Verify that the templates are copied.
	g.Eventually(func() error {
		return verifyClusterClassAndTemplates(
			env.Client,
			sourceClusterClassName,
			targetNamespace.Name,
		)
	},
		timeout,
	).Should(Succeed())
}

func verifyClusterClassAndTemplates(
	cli client.Reader,
	name,
	namespace string,
) error {
	cc := &clusterv1.ClusterClass{}
	key := client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}
	err := cli.Get(ctx, key, cc)
	if err != nil {
		return fmt.Errorf("failed to get ClusterClass %s: %w", key, err)
	}

	return walkReferences(ctx, cc, func(ctx context.Context, ref *corev1.ObjectReference) error {
		_, err := getReference(ctx, cli, ref)
		return err
	})
}

func createUniqueClusterClassAndTemplates(namespace string) (
	clusterClassName string,
	templates []client.Object,
	cleanup func() error,
	err error,
) {
	return createClusterClassAndTemplates(
		names.SimpleNameGenerator.GenerateName("test-"),
		namespace,
	)
}

func createClusterClassAndTemplates(
	prefix,
	namespace string,
) (
	clusterClassName string,
	templates []client.Object,
	cleanup func() error,
	err error,
) {
	// The below objects are created in order to feed the reconcile loop all the information it needs to create a
	// full tree of ClusterClass objects (the objects should have owner references to the ClusterClass).

	// Bootstrap templates for the workers.
	bootstrapTemplate := builder.BootstrapTemplate(namespace, prefix).Build()

	// InfraMachineTemplates for the workers and the control plane.
	infraMachineTemplateControlPlane := builder.InfrastructureMachineTemplate(
		namespace,
		fmt.Sprintf("%s-control-plane", prefix),
	).Build()
	infraMachineTemplateWorker := builder.InfrastructureMachineTemplate(
		namespace,
		fmt.Sprintf("%s-worker", prefix),
	).Build()

	// Control plane template.
	controlPlaneTemplate := builder.ControlPlaneTemplate(namespace, prefix).Build()

	// InfraClusterTemplate.
	infraClusterTemplate := builder.InfrastructureClusterTemplate(namespace, prefix).Build()

	// MachineDeploymentClasses that will be part of the ClusterClass.
	machineDeploymentClass := builder.MachineDeploymentClass(fmt.Sprintf("%s-worker", prefix)).
		WithBootstrapTemplate(bootstrapTemplate).
		WithInfrastructureTemplate(infraMachineTemplateWorker).
		Build()

	// ClusterClass.
	clusterClass := builder.ClusterClass(namespace, prefix).
		WithInfrastructureClusterTemplate(infraClusterTemplate).
		WithControlPlaneTemplate(controlPlaneTemplate).
		WithControlPlaneInfrastructureMachineTemplate(infraMachineTemplateControlPlane).
		WithWorkerMachineDeploymentClasses(machineDeploymentClass).
		Build()

	// Create the set of initObjects from the objects above to add to the API server when the test environment starts.

	templates = []client.Object{
		bootstrapTemplate,
		infraMachineTemplateWorker,
		infraMachineTemplateControlPlane,
		controlPlaneTemplate,
		infraClusterTemplate,
	}

	for _, obj := range templates {
		if err := env.CreateAndWait(ctx, obj); err != nil {
			return "", nil, nil, err
		}
	}
	if err := env.CreateAndWait(ctx, clusterClass); err != nil {
		return "", nil, nil, err
	}

	cleanup = func() error {
		for _, obj := range templates {
			if err := env.CleanupAndWait(ctx, obj); err != nil {
				return err
			}
		}
		return env.CleanupAndWait(ctx, clusterClass)
	}

	return clusterClass.Name, templates, cleanup, nil
}

func createUniqueManagedClusterClassAndTemplates(namespace string) (
	clusterClassName string,
	templates []client.Object,
	cleanup func() error,
	err error,
) {
	return createManagedClusterClassAndTemplates(
		names.SimpleNameGenerator.GenerateName("test-managed-"),
		namespace,
	)
}

// createManagedClusterClassAndTemplates creates a ClusterClass with a ControlPlane that does not have a MachineInfrastructure reference.
func createManagedClusterClassAndTemplates(
	prefix,
	namespace string,
) (
	clusterClassName string,
	templates []client.Object,
	cleanup func() error,
	err error,
) {
	// The below objects are created in order to feed the reconcile loop all the information it needs to create a
	// full tree of ClusterClass objects (the objects should have owner references to the ClusterClass).

	// Bootstrap templates for the workers.
	bootstrapTemplate := builder.BootstrapTemplate(namespace, prefix).Build()

	// InfraMachineTemplates for the workers
	infraMachineTemplateWorker := builder.InfrastructureMachineTemplate(
		namespace,
		fmt.Sprintf("%s-worker", prefix),
	).Build()

	// Control plane template.
	controlPlaneTemplate := builder.ControlPlaneTemplate(namespace, prefix).Build()

	// InfraClusterTemplate.
	infraClusterTemplate := builder.InfrastructureClusterTemplate(namespace, prefix).Build()

	// MachineDeploymentClasses that will be part of the ClusterClass.
	machineDeploymentClass := builder.MachineDeploymentClass(fmt.Sprintf("%s-worker", prefix)).
		WithBootstrapTemplate(bootstrapTemplate).
		WithInfrastructureTemplate(infraMachineTemplateWorker).
		Build()

	// ClusterClass.
	clusterClass := builder.ClusterClass(namespace, prefix).
		WithInfrastructureClusterTemplate(infraClusterTemplate).
		WithControlPlaneTemplate(controlPlaneTemplate).
		WithWorkerMachineDeploymentClasses(machineDeploymentClass).
		Build()

	// Create the set of initObjects from the objects above to add to the API server when the test environment starts.

	templates = []client.Object{
		bootstrapTemplate,
		infraMachineTemplateWorker,
		controlPlaneTemplate,
		infraClusterTemplate,
	}

	for _, obj := range templates {
		if err := env.CreateAndWait(ctx, obj); err != nil {
			return "", nil, nil, err
		}
	}
	if err := env.CreateAndWait(ctx, clusterClass); err != nil {
		return "", nil, nil, err
	}

	cleanup = func() error {
		for _, obj := range templates {
			if err := env.CleanupAndWait(ctx, obj); err != nil {
				return err
			}
		}
		return env.CleanupAndWait(ctx, clusterClass)
	}

	return clusterClass.Name, templates, cleanup, nil
}

func createTargetNamespaces(number int) ([]*corev1.Namespace, error) {
	targetNamespaces := []*corev1.Namespace{}
	for i := 0; i < number; i++ {
		targetNamespace, err := env.CreateNamespace(ctx, "target", map[string]string{
			targetNamespaceLabelKey: "",
		})
		if err != nil {
			return nil, err
		}
		targetNamespaces = append(targetNamespaces, targetNamespace)
	}
	return targetNamespaces, nil
}
