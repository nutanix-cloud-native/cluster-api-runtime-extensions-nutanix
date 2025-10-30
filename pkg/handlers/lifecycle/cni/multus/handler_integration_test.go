// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package multus

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var (
	clientScheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
	utilruntime.Must(clusterv1.AddToScheme(clientScheme))
	utilruntime.Must(caaphv1.AddToScheme(clientScheme))
}

var _ = Describe("Test Multus Handler Integration", func() {
	var (
		c              ctrlclient.Client
		cluster        *clusterv1.Cluster
		handler        *MultusHandler
		helmChartGetter *config.HelmChartGetter
	)

	BeforeEach(func(ctx SpecContext) {
		var err error
		c, err = helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		Expect(err).To(BeNil())

		// Create values template ConfigMap
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default-multus-values-template",
				Namespace: corev1.NamespaceDefault,
			},
			Data: map[string]string{
				"values.yaml": `daemonConfig:
  readinessIndicatorFile: "{{ .ReadinessSocketPath }}"
`,
			},
		}
		err = c.Create(ctx, cm)
		Expect(err).To(BeNil())

		// Initialize HelmChartGetter
		helmChartGetter = initializeHelmChartGetterFromConfigMap(ctx, c)

		// Create handler
		globalOptions := options.NewGlobalOptions()
		multusConfig := NewMultusConfig(globalOptions)
		handler = New(c, multusConfig, helmChartGetter)

		// Create test cluster with EKS infrastructure and Cilium CNI
		cluster = createTestCluster(ctx, c, &carenv1.CNI{
			Provider: v1alpha1.CNIProviderCilium,
		})
	})

	AfterEach(func(ctx SpecContext) {
		if cluster != nil {
			_ = c.Delete(ctx, cluster)
		}
		_ = c.DeleteAllOf(ctx, &corev1.ConfigMap{})
		_ = c.DeleteAllOf(ctx, &caaphv1.HelmChartProxy{})
	})

	It("should create HelmChartProxy for supported cluster with CNI", func(ctx SpecContext) {
		req := &runtimehooksv1.AfterControlPlaneInitializedRequest{
			Cluster: *cluster,
		}
		resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}

		handler.AfterControlPlaneInitialized(ctx, req, resp)

		Expect(resp.Status).To(Equal(runtimehooksv1.ResponseStatusSuccess))

		// Verify HelmChartProxy is created
		helmChartProxyKey := ctrlclient.ObjectKey{
			Name:      defaultMultusReleaseName,
			Namespace: cluster.Namespace,
		}
		helmChartProxy := &caaphv1.HelmChartProxy{}
		err := c.Get(ctx, helmChartProxyKey, helmChartProxy)
		Expect(err).To(BeNil())
		Expect(helmChartProxy.Spec.ReleaseName).To(Equal(defaultMultusReleaseName))
	})

	It("should skip deployment for unsupported cloud provider", func(ctx SpecContext) {
		// Create cluster with unsupported infrastructure
		unsupportedCluster := createTestClusterWithInfra(ctx, c, "DockerCluster", &carenv1.CNI{
			Provider: v1alpha1.CNIProviderCilium,
		})

		req := &runtimehooksv1.AfterControlPlaneInitializedRequest{
			Cluster: *unsupportedCluster,
		}
		resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}

		handler.AfterControlPlaneInitialized(ctx, req, resp)

		// Status should be empty (not failure, just skipped)
		Expect(resp.Status).To(Equal(runtimehooksv1.ResponseStatus("")))

		// Verify no HelmChartProxy was created
		helmChartProxyKey := ctrlclient.ObjectKey{
			Name:      defaultMultusReleaseName,
			Namespace: unsupportedCluster.Namespace,
		}
		helmChartProxy := &caaphv1.HelmChartProxy{}
		err := c.Get(ctx, helmChartProxyKey, helmChartProxy)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})
})

func createTestCluster(
	ctx context.Context,
	c ctrlclient.Client,
	cni *carenv1.CNI,
) *clusterv1.Cluster {
	return createTestClusterWithInfra(ctx, c, "AWSManagedCluster", cni)
}

func createTestClusterWithInfra(
	ctx context.Context,
	c ctrlclient.Client,
	infraKind string,
	cni *carenv1.CNI,
) *clusterv1.Cluster {
	cv, err := apivariables.MarshalToClusterVariable(
		v1alpha1.ClusterConfigVariableName,
		&apivariables.ClusterConfigSpec{
			Addons: &apivariables.Addons{
				GenericAddons: v1alpha1.GenericAddons{
					CNI: cni,
				},
			},
		},
	)
	Expect(err).ToNot(HaveOccurred())

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-cluster-",
			Namespace:    corev1.NamespaceDefault,
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: &corev1.ObjectReference{
				Kind: infraKind,
			},
			Topology: &clusterv1.Topology{
				Class:   "dummy-class",
				Version: "dummy-version",
				Variables: []clusterv1.ClusterVariable{
					*cv,
				},
			},
		},
	}
	err = c.Create(ctx, cluster)
	Expect(err).ToNot(HaveOccurred())

	return cluster
}

func initializeHelmChartGetterFromConfigMap(
	ctx context.Context,
	c ctrlclient.Client,
) *config.HelmChartGetter {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-helm-addons-config-",
			Namespace:    corev1.NamespaceDefault,
		},
		Data: map[string]string{
			"multus": `ChartName: multus
ChartVersion: 0.1.0
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

