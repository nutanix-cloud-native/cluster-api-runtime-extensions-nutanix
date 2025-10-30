// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package multus

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	v1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var clientScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
	utilruntime.Must(clusterv1.AddToScheme(clientScheme))
	utilruntime.Must(caaphv1.AddToScheme(clientScheme))
}

var _ = Describe("Test Multus Handler Integration", func() {
	var (
		c               ctrlclient.Client
		cluster         *clusterv1.Cluster
		handler         *MultusHandler
		helmChartGetter *config.HelmChartGetter
	)

	BeforeEach(func(ctx SpecContext) {
		var err error
		c, err = helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		Expect(err).To(BeNil())

		// Clean up any existing resources first
		_ = c.DeleteAllOf(ctx, &corev1.ConfigMap{})
		_ = c.DeleteAllOf(ctx, &caaphv1.HelmChartProxy{})
		_ = c.DeleteAllOf(ctx, &clusterv1.Cluster{})

		// Create values template ConfigMap
		// Try to delete first, then create (ignore errors)
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
		// Delete if exists (ignore error)
		_ = c.Delete(ctx, cm)
		// Create (ignore AlreadyExists - it means it's already there from previous test)
		err = c.Create(ctx, cm)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			Expect(err).To(BeNil())
		}

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
		// Start a goroutine to update HelmChartProxy status to Ready when it's created
		expectedHelmChartProxyName := fmt.Sprintf(
			"%s-%s",
			defaultMultusReleaseName,
			cluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey],
		)
		go updateHelmChartProxyStatusToReady(ctx, c, expectedHelmChartProxyName, cluster.Namespace)

		req := &runtimehooksv1.AfterControlPlaneInitializedRequest{
			Cluster: *cluster,
		}
		resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}

		handler.AfterControlPlaneInitialized(ctx, req, resp)

		Expect(resp.Status).To(Equal(runtimehooksv1.ResponseStatusSuccess))

		// Verify HelmChartProxy is created
		helmChartProxyKey := ctrlclient.ObjectKey{
			Name:      expectedHelmChartProxyName,
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
		}, "unsupported-cloud-provider-uuid")

		req := &runtimehooksv1.AfterControlPlaneInitializedRequest{
			Cluster: *unsupportedCluster,
		}
		resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}

		handler.AfterControlPlaneInitialized(ctx, req, resp)

		// Status should be empty (not failure, just skipped) - handler returns early
		// without setting status when provider is not supported
		Expect(resp.Status).To(Equal(runtimehooksv1.ResponseStatus("")))
		Expect(resp.Message).To(BeEmpty())

		// Verify no HelmChartProxy was created
		// HelmChartProxy name format: {releaseName}-{clusterUUID}
		expectedHelmChartProxyName := fmt.Sprintf(
			"%s-%s",
			defaultMultusReleaseName,
			unsupportedCluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey],
		)
		helmChartProxyKey := ctrlclient.ObjectKey{
			Name:      expectedHelmChartProxyName,
			Namespace: unsupportedCluster.Namespace,
		}
		helmChartProxy := &caaphv1.HelmChartProxy{}
		err := c.Get(ctx, helmChartProxyKey, helmChartProxy)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	It("should create HelmChartProxy for Calico CNI", func(ctx SpecContext) {
		// Create cluster with Calico CNI
		calicoCluster := createTestCluster(ctx, c, &carenv1.CNI{
			Provider: v1alpha1.CNIProviderCalico,
		})

		// Start a goroutine to update HelmChartProxy status to Ready when it's created
		expectedHelmChartProxyName := fmt.Sprintf(
			"%s-%s",
			defaultMultusReleaseName,
			calicoCluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey],
		)
		go updateHelmChartProxyStatusToReady(ctx, c, expectedHelmChartProxyName, calicoCluster.Namespace)

		req := &runtimehooksv1.AfterControlPlaneInitializedRequest{
			Cluster: *calicoCluster,
		}
		resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}

		handler.AfterControlPlaneInitialized(ctx, req, resp)

		Expect(resp.Status).To(Equal(runtimehooksv1.ResponseStatusSuccess))

		// Verify HelmChartProxy is created
		helmChartProxyKey := ctrlclient.ObjectKey{
			Name:      expectedHelmChartProxyName,
			Namespace: calicoCluster.Namespace,
		}
		helmChartProxy := &caaphv1.HelmChartProxy{}
		err := c.Get(ctx, helmChartProxyKey, helmChartProxy)
		Expect(err).To(BeNil())
		Expect(helmChartProxy.Spec.ReleaseName).To(Equal(defaultMultusReleaseName))
	})

	It("should skip deployment for unsupported CNI provider", func(ctx SpecContext) {
		// Create cluster with unsupported CNI
		unsupportedCNICluster := createTestClusterWithInfra(ctx, c, "AWSManagedCluster", &carenv1.CNI{
			Provider: "UnsupportedCNI",
		}, "unsupported-cni-provider-uuid")

		req := &runtimehooksv1.AfterControlPlaneInitializedRequest{
			Cluster: *unsupportedCNICluster,
		}
		resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}

		handler.AfterControlPlaneInitialized(ctx, req, resp)

		// Unsupported CNI provider causes templateValuesFunc to fail,
		// which causes Apply to fail, returning Failure status
		Expect(resp.Status).To(Equal(runtimehooksv1.ResponseStatusFailure))
		Expect(resp.Message).To(ContainSubstring("failed to deploy Multus"))

		// Verify no HelmChartProxy was created
		// HelmChartProxy name format: {releaseName}-{clusterUUID}
		expectedHelmChartProxyName := fmt.Sprintf(
			"%s-%s",
			defaultMultusReleaseName,
			unsupportedCNICluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey],
		)
		helmChartProxyKey := ctrlclient.ObjectKey{
			Name:      expectedHelmChartProxyName,
			Namespace: unsupportedCNICluster.Namespace,
		}
		helmChartProxy := &caaphv1.HelmChartProxy{}
		err := c.Get(ctx, helmChartProxyKey, helmChartProxy)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	It("should verify HelmChartProxy values template rendering for Cilium", func(ctx SpecContext) {
		// Start a goroutine to update HelmChartProxy status to Ready when it's created
		expectedHelmChartProxyName := fmt.Sprintf(
			"%s-%s",
			defaultMultusReleaseName,
			cluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey],
		)
		go updateHelmChartProxyStatusToReady(ctx, c, expectedHelmChartProxyName, cluster.Namespace)

		req := &runtimehooksv1.AfterControlPlaneInitializedRequest{
			Cluster: *cluster,
		}
		resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}

		handler.AfterControlPlaneInitialized(ctx, req, resp)

		Expect(resp.Status).To(Equal(runtimehooksv1.ResponseStatusSuccess))

		// Verify HelmChartProxy values contain correct socket path for Cilium
		helmChartProxyKey := ctrlclient.ObjectKey{
			Name:      expectedHelmChartProxyName,
			Namespace: cluster.Namespace,
		}
		helmChartProxy := &caaphv1.HelmChartProxy{}
		err := c.Get(ctx, helmChartProxyKey, helmChartProxy)
		Expect(err).To(BeNil())

		// Verify values contain Cilium socket path
		Expect(helmChartProxy.Spec.ValuesTemplate).To(ContainSubstring("/run/cilium/cilium.sock"))
	})

	It("should verify HelmChartProxy values template rendering for Calico", func(ctx SpecContext) {
		// Create cluster with Calico CNI
		calicoCluster := createTestCluster(ctx, c, &carenv1.CNI{
			Provider: v1alpha1.CNIProviderCalico,
		})

		// Start a goroutine to update HelmChartProxy status to Ready when it's created
		expectedHelmChartProxyName := fmt.Sprintf(
			"%s-%s",
			defaultMultusReleaseName,
			calicoCluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey],
		)
		go updateHelmChartProxyStatusToReady(ctx, c, expectedHelmChartProxyName, calicoCluster.Namespace)

		req := &runtimehooksv1.AfterControlPlaneInitializedRequest{
			Cluster: *calicoCluster,
		}
		resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}

		handler.AfterControlPlaneInitialized(ctx, req, resp)

		Expect(resp.Status).To(Equal(runtimehooksv1.ResponseStatusSuccess))

		// Verify HelmChartProxy values contain correct socket path for Calico
		helmChartProxyKey := ctrlclient.ObjectKey{
			Name:      expectedHelmChartProxyName,
			Namespace: calicoCluster.Namespace,
		}
		helmChartProxy := &caaphv1.HelmChartProxy{}
		err := c.Get(ctx, helmChartProxyKey, helmChartProxy)
		Expect(err).To(BeNil())

		// Verify values contain Calico socket path
		Expect(helmChartProxy.Spec.ValuesTemplate).To(ContainSubstring("/var/run/calico/cni-server.sock"))
	})

	It("should handle missing HelmChart config gracefully", func(ctx SpecContext) {
		// Create handler without HelmChart config
		clientWithoutConfig, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		Expect(err).To(BeNil())

		// Delete helm config ConfigMap if it exists
		_ = c.DeleteAllOf(ctx, &corev1.ConfigMap{})

		globalOptions := options.NewGlobalOptions()
		multusConfig := NewMultusConfig(globalOptions)
		// Create HelmChartGetter that will fail to find config
		helmChartGetterWithoutConfig := config.NewHelmChartGetterFromConfigMap(
			"non-existent-config",
			corev1.NamespaceDefault,
			clientWithoutConfig,
		)
		handlerWithoutConfig := New(clientWithoutConfig, multusConfig, helmChartGetterWithoutConfig)

		req := &runtimehooksv1.AfterControlPlaneInitializedRequest{
			Cluster: *cluster,
		}
		resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}

		handlerWithoutConfig.AfterControlPlaneInitialized(ctx, req, resp)

		// Should return failure status
		Expect(resp.Status).To(Equal(runtimehooksv1.ResponseStatusFailure))
		Expect(resp.Message).To(ContainSubstring("failed to get configuration"))
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
	clusterUUID ...string,
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

	// Set provider label based on infrastructure kind
	providerLabel := ""
	switch infraKind {
	case "AWSManagedCluster":
		providerLabel = "eks"
	case "DockerCluster":
		providerLabel = "docker"
	case "NutanixCluster":
		providerLabel = "nutanix"
	}

	// Use provided UUID or generate a unique one
	uuid := "test-cluster-uuid"
	if len(clusterUUID) > 0 && clusterUUID[0] != "" {
		uuid = clusterUUID[0]
	} else {
		uuid = fmt.Sprintf("test-cluster-uuid-%d", time.Now().UnixNano())
	}

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-cluster-",
			Namespace:    corev1.NamespaceDefault,
			Annotations: map[string]string{
				v1alpha1.ClusterUUIDAnnotationKey: uuid,
			},
			Labels: map[string]string{},
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
	if providerLabel != "" {
		cluster.Labels[clusterv1.ProviderNameLabel] = providerLabel
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

// updateHelmChartProxyStatusToReady polls for the HelmChartProxy and updates its status to Ready
// This allows the WithDefaultWaiter() to succeed in integration tests
func updateHelmChartProxyStatusToReady(
	ctx context.Context,
	c ctrlclient.Client,
	name, namespace string,
) {
	key := types.NamespacedName{Name: name, Namespace: namespace}

	// Poll until the HelmChartProxy is created and update status
	// Use a more aggressive polling interval to catch it quickly
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	// Keep trying until context is cancelled or update succeeds
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hcp := &caaphv1.HelmChartProxy{}
			if err := c.Get(ctx, key, hcp); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				// If there's a non-NotFound error, log and retry
				continue
			}

			// Update status to Ready - need to get fresh resourceVersion
			// Set ObservedGeneration to match Generation
			hcpCopy := hcp.DeepCopy()
			hcpCopy.Status.ObservedGeneration = hcpCopy.Generation
			conditions.MarkTrue(hcpCopy, v1beta1.ReadyCondition)

			// Use Patch to avoid resourceVersion conflicts
			patch := ctrlclient.MergeFrom(hcp)
			if err := c.Status().Patch(ctx, hcpCopy, patch); err == nil {
				return
			}
			// If update fails, continue polling - might be a resourceVersion conflict
		}
	}
}
