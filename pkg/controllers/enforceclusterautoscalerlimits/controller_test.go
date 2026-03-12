// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package enforceclusterautoscalerlimits

import (
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/test/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newMachineDeploymentWithTemplates(
	namespace, name, clusterName string,
	buildOpts ...func(*builder.MachineDeploymentBuilder) *builder.MachineDeploymentBuilder,
) (*clusterv1beta2.MachineDeployment, []client.Object) {
	bootstrapTemplate := builder.BootstrapTemplate(namespace, name).Build()
	infraTemplate := builder.InfrastructureMachineTemplate(namespace, name).Build()
	b := builder.MachineDeployment(namespace, name).
		WithClusterName(clusterName).
		WithBootstrapTemplate(bootstrapTemplate).
		WithInfrastructureTemplate(infraTemplate)
	for _, opt := range buildOpts {
		b = opt(b)
	}
	templates := []client.Object{bootstrapTemplate, infraTemplate}
	return b.Build(), templates
}

func TestReconcileMachineDeploymentWithNoReplicasOrClusterAutoscalerAnnotations(t *testing.T) {
	g := NewWithT(t)
	timeout := 5 * time.Second

	sourceMachineDeployment, templates := newMachineDeploymentWithTemplates(
		metav1.NamespaceDefault,
		"test-md",
		"test",
	)
	for _, obj := range templates {
		g.Expect(env.CreateAndWait(ctx, obj)).To(Succeed())
	}
	g.Expect(env.Client.Create(ctx, sourceMachineDeployment)).To(Succeed())
	defer func() {
		g.Expect(env.CleanupAndWait(ctx, sourceMachineDeployment)).To(Succeed())
		for _, obj := range templates {
			g.Expect(env.CleanupAndWait(ctx, obj)).To(Succeed())
		}
	}()

	g.Eventually(func() error {
		return env.Client.Get(
			ctx,
			client.ObjectKeyFromObject(sourceMachineDeployment),
			&clusterv1beta2.MachineDeployment{},
		)
	}, timeout).To(Succeed())

	g.Consistently(func() error {
		return verifyMachineDeploymentReplicas(
			env.Client,
			client.ObjectKeyFromObject(sourceMachineDeployment),
			nil,
		)
	},
		timeout,
	).Should(Succeed())
}

func TestReconcileMachineDeploymentWithReplicasOnly(t *testing.T) {
	g := NewWithT(t)
	timeout := 5 * time.Second

	sourceMachineDeployment, templates := newMachineDeploymentWithTemplates(
		metav1.NamespaceDefault,
		"test-md",
		"test",
		func(b *builder.MachineDeploymentBuilder) *builder.MachineDeploymentBuilder { return b.WithReplicas(5) },
	)
	for _, obj := range templates {
		g.Expect(env.CreateAndWait(ctx, obj)).To(Succeed())
	}
	g.Expect(env.Client.Create(ctx, sourceMachineDeployment)).To(Succeed())
	defer func() {
		g.Expect(env.CleanupAndWait(ctx, sourceMachineDeployment)).To(Succeed())
		for _, obj := range templates {
			g.Expect(env.CleanupAndWait(ctx, obj)).To(Succeed())
		}
	}()

	g.Eventually(func() error {
		return env.Client.Get(
			ctx,
			client.ObjectKeyFromObject(sourceMachineDeployment),
			&clusterv1beta2.MachineDeployment{},
		)
	}).To(Succeed())

	g.Consistently(func() error {
		return verifyMachineDeploymentReplicas(
			env.Client,
			client.ObjectKeyFromObject(sourceMachineDeployment),
			ptr.To[int32](5),
		)
	},
		timeout,
	).Should(Succeed())
}

func TestReconcileMachineDeploymentWithReplicasAndMinAnnotationOnly(t *testing.T) {
	g := NewWithT(t)
	timeout := 5 * time.Second

	sourceMachineDeployment, templates := newMachineDeploymentWithTemplates(
		metav1.NamespaceDefault,
		"test-md",
		"test",
		func(b *builder.MachineDeploymentBuilder) *builder.MachineDeploymentBuilder {
			return b.
				WithReplicas(5).
				WithAnnotations(map[string]string{
					clusterv1beta2.AutoscalerMinSizeAnnotation: "12",
				})
		},
	)
	for _, obj := range templates {
		g.Expect(env.CreateAndWait(ctx, obj)).To(Succeed())
	}
	g.Expect(env.Client.Create(ctx, sourceMachineDeployment)).To(Succeed())
	defer func() {
		g.Expect(env.CleanupAndWait(ctx, sourceMachineDeployment)).To(Succeed())
		for _, obj := range templates {
			g.Expect(env.CleanupAndWait(ctx, obj)).To(Succeed())
		}
	}()

	g.Eventually(func() error {
		return env.Client.Get(
			ctx,
			client.ObjectKeyFromObject(sourceMachineDeployment),
			&clusterv1beta2.MachineDeployment{},
		)
	}, timeout).To(Succeed())

	g.Consistently(func() error {
		return verifyMachineDeploymentReplicas(
			env.Client,
			client.ObjectKeyFromObject(sourceMachineDeployment),
			ptr.To[int32](5),
		)
	},
		timeout,
	).Should(Succeed())
}

func TestReconcileMachineDeploymentWithReplicasAndMaxAnnotationOnly(t *testing.T) {
	g := NewWithT(t)
	timeout := 5 * time.Second

	sourceMachineDeployment, templates := newMachineDeploymentWithTemplates(
		metav1.NamespaceDefault,
		"test-md",
		"test",
		func(b *builder.MachineDeploymentBuilder) *builder.MachineDeploymentBuilder {
			return b.
				WithReplicas(5).
				WithAnnotations(map[string]string{
					clusterv1beta2.AutoscalerMaxSizeAnnotation: "3",
				})
		},
	)
	for _, obj := range templates {
		g.Expect(env.CreateAndWait(ctx, obj)).To(Succeed())
	}
	g.Expect(env.Client.Create(ctx, sourceMachineDeployment)).To(Succeed())
	defer func() {
		g.Expect(env.CleanupAndWait(ctx, sourceMachineDeployment)).To(Succeed())
		for _, obj := range templates {
			g.Expect(env.CleanupAndWait(ctx, obj)).To(Succeed())
		}
	}()

	g.Eventually(func() error {
		return env.Client.Get(
			ctx,
			client.ObjectKeyFromObject(sourceMachineDeployment),
			&clusterv1beta2.MachineDeployment{},
		)
	}, timeout).To(Succeed())

	g.Consistently(func() error {
		return verifyMachineDeploymentReplicas(
			env.Client,
			client.ObjectKeyFromObject(sourceMachineDeployment),
			ptr.To[int32](5),
		)
	},
		timeout,
	).Should(Succeed())
}

func TestReconcileMachineDeploymentWithReplicasWithinMinMaxBounds(t *testing.T) {
	g := NewWithT(t)
	timeout := 5 * time.Second

	sourceMachineDeployment, templates := newMachineDeploymentWithTemplates(
		metav1.NamespaceDefault,
		"test-md",
		"test",
		func(b *builder.MachineDeploymentBuilder) *builder.MachineDeploymentBuilder {
			return b.
				WithReplicas(5).
				WithAnnotations(map[string]string{
					clusterv1beta2.AutoscalerMinSizeAnnotation: "3",
					clusterv1beta2.AutoscalerMaxSizeAnnotation: "12",
				})
		},
	)
	for _, obj := range templates {
		g.Expect(env.CreateAndWait(ctx, obj)).To(Succeed())
	}
	g.Expect(env.Client.Create(ctx, sourceMachineDeployment)).To(Succeed())
	defer func() {
		g.Expect(env.CleanupAndWait(ctx, sourceMachineDeployment)).To(Succeed())
		for _, obj := range templates {
			g.Expect(env.CleanupAndWait(ctx, obj)).To(Succeed())
		}
	}()

	g.Eventually(func() error {
		return env.Client.Get(
			ctx,
			client.ObjectKeyFromObject(sourceMachineDeployment),
			&clusterv1beta2.MachineDeployment{},
		)
	}, timeout).To(Succeed())

	g.Consistently(func() error {
		return verifyMachineDeploymentReplicas(
			env.Client,
			client.ObjectKeyFromObject(sourceMachineDeployment),
			ptr.To[int32](5),
		)
	},
		timeout,
	).Should(Succeed())
}

func TestReconcileMachineDeploymentWithReplicasLessThanMinBound(t *testing.T) {
	g := NewWithT(t)
	timeout := 5 * time.Second

	sourceMachineDeployment, templates := newMachineDeploymentWithTemplates(
		metav1.NamespaceDefault,
		"test-md",
		"test",
		func(b *builder.MachineDeploymentBuilder) *builder.MachineDeploymentBuilder {
			return b.
				WithReplicas(5).
				WithAnnotations(map[string]string{
					clusterv1beta2.AutoscalerMinSizeAnnotation: "7",
					clusterv1beta2.AutoscalerMaxSizeAnnotation: "12",
				})
		},
	)
	for _, obj := range templates {
		g.Expect(env.CreateAndWait(ctx, obj)).To(Succeed())
	}
	g.Expect(env.Client.Create(ctx, sourceMachineDeployment)).To(Succeed())
	defer func() {
		g.Expect(env.CleanupAndWait(ctx, sourceMachineDeployment)).To(Succeed())
		for _, obj := range templates {
			g.Expect(env.CleanupAndWait(ctx, obj)).To(Succeed())
		}
	}()

	g.Eventually(func() error {
		return verifyMachineDeploymentReplicas(
			env.Client,
			client.ObjectKeyFromObject(sourceMachineDeployment),
			nil,
		)
	},
		timeout,
	).Should(Succeed())
}

func TestReconcileMachineDeploymentWithReplicasMoreThanMaxBound(t *testing.T) {
	g := NewWithT(t)
	timeout := 5 * time.Second

	sourceMachineDeployment, templates := newMachineDeploymentWithTemplates(
		metav1.NamespaceDefault,
		"test-md",
		"test",
		func(b *builder.MachineDeploymentBuilder) *builder.MachineDeploymentBuilder {
			return b.
				WithReplicas(15).
				WithAnnotations(map[string]string{
					clusterv1beta2.AutoscalerMinSizeAnnotation: "7",
					clusterv1beta2.AutoscalerMaxSizeAnnotation: "12",
				})
		},
	)
	for _, obj := range templates {
		g.Expect(env.CreateAndWait(ctx, obj)).To(Succeed())
	}
	g.Expect(env.Client.Create(ctx, sourceMachineDeployment)).To(Succeed())
	defer func() {
		g.Expect(env.CleanupAndWait(ctx, sourceMachineDeployment)).To(Succeed())
		for _, obj := range templates {
			g.Expect(env.CleanupAndWait(ctx, obj)).To(Succeed())
		}
	}()

	g.Eventually(func() error {
		return verifyMachineDeploymentReplicas(
			env.Client,
			client.ObjectKeyFromObject(sourceMachineDeployment),
			nil,
		)
	},
		timeout,
	).Should(Succeed())
}

func TestReconcileMachineDeploymentWithInvalidClusterAutoscalerAnnotations(t *testing.T) {
	g := NewWithT(t)
	timeout := 5 * time.Second

	sourceMachineDeployment, templates := newMachineDeploymentWithTemplates(
		metav1.NamespaceDefault,
		"test-md",
		"test",
		func(b *builder.MachineDeploymentBuilder) *builder.MachineDeploymentBuilder {
			return b.
				WithReplicas(1).
				WithAnnotations(map[string]string{
					clusterv1beta2.AutoscalerMaxSizeAnnotation: "3",
					clusterv1beta2.AutoscalerMinSizeAnnotation: "7",
				})
		},
	)
	for _, obj := range templates {
		g.Expect(env.CreateAndWait(ctx, obj)).To(Succeed())
	}
	g.Expect(env.Client.Create(ctx, sourceMachineDeployment)).To(Succeed())
	defer func() {
		g.Expect(env.CleanupAndWait(ctx, sourceMachineDeployment)).To(Succeed())
		for _, obj := range templates {
			g.Expect(env.CleanupAndWait(ctx, obj)).To(Succeed())
		}
	}()

	g.Eventually(func() error {
		return env.Client.Get(
			ctx,
			client.ObjectKeyFromObject(sourceMachineDeployment),
			&clusterv1beta2.MachineDeployment{},
		)
	}).To(Succeed())

	g.Consistently(func() error {
		return verifyMachineDeploymentReplicas(
			env.Client,
			client.ObjectKeyFromObject(sourceMachineDeployment),
			ptr.To[int32](1),
		)
	},
		timeout,
	).Should(Succeed())
}

func verifyMachineDeploymentReplicas(
	c client.Client,
	key client.ObjectKey,
	expectedReplicas *int32,
) error {
	var md clusterv1beta2.MachineDeployment
	if err := c.Get(ctx, key, &md); err != nil {
		return fmt.Errorf("failed to get MachineDeployment %s: %w", key, err)
	}

	if expectedReplicas == nil && md.Spec.Replicas != nil {
		return fmt.Errorf("expected replicas to be nil, but got %d", *md.Spec.Replicas)
	}

	if expectedReplicas != nil && md.Spec.Replicas == nil {
		return fmt.Errorf("expected %d replicas, but got nil", *expectedReplicas)
	}

	if expectedReplicas != nil && *md.Spec.Replicas != *expectedReplicas {
		return fmt.Errorf("expected %d replicas, but got %d", *expectedReplicas, *md.Spec.Replicas)
	}

	return nil
}
