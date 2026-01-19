// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package syncer

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/feature"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var _ = Describe("Test Syncer", func() {
	clientScheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
	utilruntime.Must(clusterv1.AddToScheme(clientScheme))
	utilruntime.Must(caaphv1.AddToScheme(clientScheme))

	It("Should create HelmChartProxy and then delete it", func(ctx SpecContext) {
		t := GinkgoT()
		featuregatetesting.SetFeatureGateDuringTest(
			t,
			feature.Gates,
			feature.SynchronizeWorkloadClusterRegistry,
			true,
		)

		c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		Expect(err).To(BeNil())

		// Create a management cluster.
		managementCluster := createTestManagementCluster(
			ctx, c,
			&carenv1.DockerClusterConfigSpec{
				Addons: &carenv1.DockerAddons{
					GenericAddons: carenv1.GenericAddons{
						CNI: &carenv1.CNI{},
						Registry: &carenv1.RegistryAddon{
							Provider: carenv1.RegistryProviderCNCFDistribution,
						},
					},
				},
			},
		)
		// Create a workload cluster.
		workloadCluster := createTestCluster(
			ctx, c,
			map[string]string{
				carenv1.ClusterUUIDAnnotationKey: "123-456-789",
			},
			&carenv1.DockerClusterConfigSpec{
				Addons: &carenv1.DockerAddons{
					GenericAddons: carenv1.GenericAddons{
						CNI: &carenv1.CNI{},
						Registry: &carenv1.RegistryAddon{
							Provider: carenv1.RegistryProviderCNCFDistribution,
						},
					},
				},
			},
		)
		// Create values template ConfigMap.
		createDefaultValuesTemplateConfigMapName(ctx, c)

		helmChartInfoGetter := initializeHelmChartGetterFromConfigMap(ctx, c)
		cfg := &Config{GlobalOptions: options.NewGlobalOptions()}

		syncer := New(c, cfg, helmChartInfoGetter)
		err = syncer.Apply(ctx, workloadCluster, logr.Discard())
		Expect(err).To(BeNil())

		// Verify that the registry syncer HelmChartProxy is created.
		registrySyncerHelmChartProxyKey := ctrlclient.ObjectKey{
			Name:      addonResourceNameForCluster(workloadCluster),
			Namespace: corev1.NamespaceDefault,
		}
		registrySyncerHelmChartProxy := &caaphv1.HelmChartProxy{}
		err = c.Get(ctx, registrySyncerHelmChartProxyKey, registrySyncerHelmChartProxy)
		Expect(err).To(BeNil())

		// Verify the HelmChartProxy fields are set correctly.
		expectedReleaseName := addonResourceNameForCluster(workloadCluster)
		Expect(registrySyncerHelmChartProxy.Spec.ReleaseName).To(Equal(expectedReleaseName))

		expectedMatchLabels := map[string]string{clusterv1.ClusterNameLabel: managementCluster.Name}
		Expect(registrySyncerHelmChartProxy.Spec.ClusterSelector.MatchLabels).To(Equal(expectedMatchLabels))

		// Run the cleanup and verify that the HelmChartProxy is deleted.
		err = syncer.Cleanup(ctx, workloadCluster, logr.Discard())
		Expect(err).To(BeNil())

		err = c.Get(ctx, registrySyncerHelmChartProxyKey, registrySyncerHelmChartProxy)
		Expect(err).ToNot(BeNil())
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})
	AfterEach(func(ctx SpecContext) {
		c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		Expect(err).To(BeNil())

		Expect(c.DeleteAllOf(ctx, &corev1.Node{})).To(Succeed())
	})
})

func createTestManagementCluster(
	ctx context.Context,
	c ctrlclient.Client,
	clusterConfigSpec *carenv1.DockerClusterConfigSpec,
) *clusterv1.Cluster {
	managementCluster := createTestCluster(
		ctx, c,
		nil,
		clusterConfigSpec,
	)

	// Create a node in the management cluster that refers to the management cluster.
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-node",
			Annotations: map[string]string{
				clusterv1.ClusterNameAnnotation:      managementCluster.Name,
				clusterv1.ClusterNamespaceAnnotation: managementCluster.Namespace,
			},
		},
	}
	Expect(c.Create(ctx, node)).To(Succeed())

	return managementCluster
}

func createTestCluster(
	ctx context.Context,
	c ctrlclient.Client,
	annotations map[string]string,
	clusterConfigSpec *carenv1.DockerClusterConfigSpec,
) *clusterv1.Cluster {
	variable, err := variables.MarshalToClusterVariable(carenv1.ClusterConfigVariableName, clusterConfigSpec)
	Expect(err).ToNot(HaveOccurred())
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-cluster-",
			Annotations:  annotations,
			Namespace:    metav1.NamespaceDefault,
		},
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: &clusterv1.ClusterNetwork{
				Services: &clusterv1.NetworkRanges{
					CIDRBlocks: []string{
						"192.168.0.0/16",
					},
				},
			},
			Topology: &clusterv1.Topology{
				Class:   "dummy-class",
				Version: "dummy-version",
				Variables: []clusterv1.ClusterVariable{
					*variable,
				},
			},
		},
	}
	Expect(c.Create(ctx, cluster)).To(Succeed())

	return cluster
}

func createDefaultValuesTemplateConfigMapName(
	ctx context.Context,
	c ctrlclient.Client,
) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultValuesTemplateConfigMapName,
			Namespace: corev1.NamespaceDefault,
		},
		Data: map[string]string{
			"values.yaml": "",
		},
	}
	err := c.Create(ctx, cm)
	Expect(err).To(BeNil())
}

func initializeHelmChartGetterFromConfigMap(
	ctx context.Context,
	c ctrlclient.Client,
) *config.HelmChartGetter {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-helm-addons-config",
			Namespace:    corev1.NamespaceDefault,
		},
		Data: map[string]string{
			"registry-syncer": `ChartName: registry-syncer
ChartVersion: 0.0.0
RepositoryURL: 'oci://helm-repository.default.svc/charts'`,
		},
	}
	err := c.Create(ctx, cm)
	Expect(err).To(BeNil())

	return config.NewHelmChartGetterFromConfigMap(
		cm.Name,
		corev1.NamespaceDefault,
		c,
	)
}
