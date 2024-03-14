//go:build e2e

// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterctlv1 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SelfHostedSpecInput is the input for SelfHostedSpec.
type SelfHostedSpecInput struct {
	E2EConfig             *clusterctl.E2EConfig
	ClusterctlConfigPath  string
	BootstrapClusterProxy framework.ClusterProxy
	ArtifactFolder        string
	SkipCleanup           bool
	ControlPlaneWaiters   clusterctl.ControlPlaneWaiters
	Flavor                string

	// InfrastructureProviders specifies the infrastructure to use for clusterctl
	// operations (Example: get cluster templates).
	// Note: In most cases this need not be specified. It only needs to be specified when
	// multiple infrastructure providers (ex: CAPD + in-memory) are installed on the cluster as clusterctl will not be
	// able to identify the default.
	InfrastructureProvider *string

	// SkipUpgrade skip the upgrade of the self-hosted clusters kubernetes version.
	// If true, the variable KUBERNETES_VERSION is expected to be set.
	// If false, the variables KUBERNETES_VERSION_UPGRADE_FROM, KUBERNETES_VERSION_UPGRADE_TO,
	// ETCD_VERSION_UPGRADE_TO and COREDNS_VERSION_UPGRADE_TO are expected to be set.
	// There are also (optional) variables CONTROL_PLANE_MACHINE_TEMPLATE_UPGRADE_TO and
	// WORKERS_MACHINE_TEMPLATE_UPGRADE_TO to change the infrastructure machine template
	// during the upgrade. Note that these templates need to have the clusterctl.cluster.x-k8s.io/move
	// label in order to be moved to the self hosted cluster (since they are not part of the owner chain).
	SkipUpgrade bool

	// ControlPlaneMachineCount is used in `config cluster` to configure the count of the control plane machines used in
	// the test.
	// Default is 1.
	ControlPlaneMachineCount *int64

	// WorkerMachineCount is used in `config cluster` to configure the count of the worker machines used in the test.
	// NOTE: If the WORKER_MACHINE_COUNT var is used multiple times in the cluster template, the absolute count of
	// worker machines is a multiple of WorkerMachineCount.
	// Default is 1.
	WorkerMachineCount *int64

	// PostClusterMoved is a function that is called after the cluster is moved to self-hosted.
	PostClusterMoved func(proxy framework.ClusterProxy, cluster *clusterv1.Cluster)
}

// SelfHostedSpec implements a test that verifies Cluster API creating a cluster, pivoting to a self-hosted cluster.
func SelfHostedSpec(ctx context.Context, inputGetter func() SelfHostedSpecInput) {
	var (
		specName         = "self-hosted"
		input            SelfHostedSpecInput
		namespace        *corev1.Namespace
		cancelWatches    context.CancelFunc
		clusterResources *clusterctl.ApplyClusterTemplateAndWaitResult

		selfHostedClusterProxy  framework.ClusterProxy
		selfHostedNamespace     *corev1.Namespace
		selfHostedCancelWatches context.CancelFunc
		selfHostedCluster       *clusterv1.Cluster

		controlPlaneMachineCount int64
		workerMachineCount       int64

		kubernetesVersion string
	)

	BeforeEach(func() {
		Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)
		input = inputGetter()
		Expect(input.E2EConfig).ToNot(
			BeNil(),
			"Invalid argument. input.E2EConfig can't be nil when calling %s spec", specName,
		)
		Expect(input.ClusterctlConfigPath).To(
			BeAnExistingFile(),
			"Invalid argument. input.ClusterctlConfigPath must be an existing file when calling %s spec", specName,
		)
		Expect(input.BootstrapClusterProxy).ToNot(
			BeNil(),
			"Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName,
		)
		Expect(os.MkdirAll(input.ArtifactFolder, 0o750)).To(
			Succeed(),
			"Invalid argument. input.ArtifactFolder can't be created for %s spec", specName,
		)

		// Use KubernetesVersion if no upgrade step is defined by test input.
		Expect(input.E2EConfig.Variables).To(HaveKey(capie2e.KubernetesVersion))
		kubernetesVersion = input.E2EConfig.GetVariable(capie2e.KubernetesVersion)

		// Setup a Namespace where to host objects for this spec and create a watcher for the namespace events.
		namespace, cancelWatches = setupSpecNamespace(
			ctx,
			specName,
			input.BootstrapClusterProxy,
			input.ArtifactFolder,
		)
		clusterResources = new(clusterctl.ApplyClusterTemplateAndWaitResult)

		if input.ControlPlaneMachineCount == nil {
			controlPlaneMachineCount = 1
		} else {
			controlPlaneMachineCount = *input.ControlPlaneMachineCount
		}

		if input.WorkerMachineCount == nil {
			workerMachineCount = 1
		} else {
			workerMachineCount = *input.WorkerMachineCount
		}
	})

	It("Should pivot the bootstrap cluster to a self-hosted cluster", func() {
		By("Creating a workload cluster")

		workloadClusterName := fmt.Sprintf("%s-%s", specName, util.RandomString(6))
		clusterctlVariables := map[string]string{}

		// In case the infrastructure-docker provider is installed, ensure to add the preload images variable to load the
		// controller images into the nodes.
		// NOTE: we are checking the bootstrap cluster and assuming the workload cluster will be on the same infrastructure
		// provider. Also, given that we use it to set a variable, then it is up to cluster templates to use it or not.
		hasDockerInfrastructureProvider := hasProvider(
			ctx,
			input.BootstrapClusterProxy.GetClient(),
			"infrastructure-docker",
		)

		// In case the infrastructure-docker provider is installed, ensure to add the preload images variable to load the
		// controller images into the nodes.
		if hasDockerInfrastructureProvider {
			images := []string{}
			for _, image := range input.E2EConfig.Images {
				images = append(images, fmt.Sprintf("%q", image.Name))
			}
			clusterctlVariables["DOCKER_PRELOAD_IMAGES"] = `[` + strings.Join(images, ",") + `]`
		}

		infrastructureProvider := clusterctl.DefaultInfrastructureProvider
		if input.InfrastructureProvider != nil {
			infrastructureProvider = *input.InfrastructureProvider
		}
		clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
			ClusterProxy: input.BootstrapClusterProxy,
			ConfigCluster: clusterctl.ConfigClusterInput{
				LogFolder: filepath.Join(
					input.ArtifactFolder,
					"clusters",
					input.BootstrapClusterProxy.GetName(),
				),
				ClusterctlConfigPath:     input.ClusterctlConfigPath,
				KubeconfigPath:           input.BootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   infrastructureProvider,
				Flavor:                   input.Flavor,
				Namespace:                namespace.Name,
				ClusterName:              workloadClusterName,
				KubernetesVersion:        kubernetesVersion,
				ControlPlaneMachineCount: &controlPlaneMachineCount,
				WorkerMachineCount:       &workerMachineCount,
				ClusterctlVariables:      clusterctlVariables,
			},
			ControlPlaneWaiters:     input.ControlPlaneWaiters,
			WaitForClusterIntervals: input.E2EConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: input.E2EConfig.GetIntervals(
				specName,
				"wait-control-plane",
			),
			WaitForMachineDeployments: input.E2EConfig.GetIntervals(
				specName,
				"wait-worker-nodes",
			),
		}, clusterResources)

		if infrastructureProvider == "docker" {
			By("Loading CAPI runtime extensions image to the workload cluster")
			Expect(bootstrap.LoadImagesToKindCluster(ctx, bootstrap.LoadImagesToKindClusterInput{
				Name:   workloadClusterName,
				Images: input.E2EConfig.Images,
			})).To(Succeed())
		}

		By("Turning the workload cluster into a management cluster")
		cluster := clusterResources.Cluster
		// Get a ClusterBroker so we can interact with the workload cluster
		selfHostedClusterProxy = input.BootstrapClusterProxy.GetWorkloadCluster(
			ctx,
			cluster.Namespace,
			cluster.Name,
			framework.WithMachineLogCollector(input.BootstrapClusterProxy.GetLogCollector()),
		)

		capie2e.Byf("Creating a namespace for hosting the %s test spec", specName)
		selfHostedNamespace, selfHostedCancelWatches = framework.CreateNamespaceAndWatchEvents(
			ctx,
			framework.CreateNamespaceAndWatchEventsInput{
				Creator:             selfHostedClusterProxy.GetClient(),
				ClientSet:           selfHostedClusterProxy.GetClientSet(),
				Name:                namespace.Name,
				LogFolder:           filepath.Join(input.ArtifactFolder, "clusters", "bootstrap"),
				IgnoreAlreadyExists: true,
			},
		)

		By("Loading CAPI runtime extensions image to the workload cluster")

		By("Initializing the workload cluster")
		// watchesCtx is used in log streaming to be able to get canceld via cancelWatches after ending the test suite.
		watchesCtx, cancelWatches := context.WithCancel(ctx)
		defer cancelWatches()
		clusterctl.InitManagementClusterAndWatchControllerLogs(
			watchesCtx,
			clusterctl.InitManagementClusterAndWatchControllerLogsInput{
				ClusterProxy:              selfHostedClusterProxy,
				ClusterctlConfigPath:      input.ClusterctlConfigPath,
				InfrastructureProviders:   input.E2EConfig.InfrastructureProviders(),
				IPAMProviders:             input.E2EConfig.IPAMProviders(),
				RuntimeExtensionProviders: input.E2EConfig.RuntimeExtensionProviders(),
				AddonProviders:            input.E2EConfig.AddonProviders(),
				LogFolder: filepath.Join(
					input.ArtifactFolder,
					"clusters",
					cluster.Name,
				),
			},
			input.E2EConfig.GetIntervals(specName, "wait-controllers")...,
		)

		By("Ensure API servers are stable before doing move")
		// Nb. This check was introduced to prevent doing move to self-hosted in an aggressive way and thus avoid flakes.
		// More specifically, we were observing the test failing to get objects from the API server during move, so we
		// are now testing the API servers are stable before starting move.
		Consistently(func() error {
			kubeSystem := &corev1.Namespace{}
			return input.BootstrapClusterProxy.GetClient().
				Get(ctx, client.ObjectKey{Name: "kube-system"}, kubeSystem)
		}, "5s", "100ms").Should(BeNil(), "Failed to assert bootstrap API server stability")
		Consistently(func() error {
			kubeSystem := &corev1.Namespace{}
			return selfHostedClusterProxy.GetClient().
				Get(ctx, client.ObjectKey{Name: "kube-system"}, kubeSystem)
		}, "5s", "100ms").Should(BeNil(), "Failed to assert self-hosted API server stability")

		By("Moving the cluster to self hosted")
		clusterctl.Move(ctx, clusterctl.MoveInput{
			LogFolder:            filepath.Join(input.ArtifactFolder, "clusters", "bootstrap"),
			ClusterctlConfigPath: input.ClusterctlConfigPath,
			FromKubeconfigPath:   input.BootstrapClusterProxy.GetKubeconfigPath(),
			ToKubeconfigPath:     selfHostedClusterProxy.GetKubeconfigPath(),
			Namespace:            namespace.Name,
		})

		By("Waiting for the cluster to be reconciled after moving to self hosted")
		selfHostedCluster = framework.DiscoveryAndWaitForCluster(
			ctx,
			framework.DiscoveryAndWaitForClusterInput{
				Getter:    selfHostedClusterProxy.GetClient(),
				Namespace: selfHostedNamespace.Name,
				Name:      cluster.Name,
			},
			input.E2EConfig.GetIntervals(specName, "wait-cluster")...)

		if input.PostClusterMoved != nil {
			By("Running the post-cluster moved function")
			input.PostClusterMoved(
				selfHostedClusterProxy,
				selfHostedCluster,
			)
		}

		By("PASSED!")
	})

	AfterEach(func() {
		if selfHostedNamespace != nil {
			// Dump all Cluster API related resources to artifacts before pivoting back.
			dumpAllResources(
				ctx,
				selfHostedClusterProxy,
				input.ArtifactFolder,
				namespace,
				clusterResources.Cluster,
			)
		}
		if selfHostedCluster != nil {
			By("Ensure API servers are stable before doing move")
			// Nb. This check was introduced to prevent doing move back to bootstrap in an aggressive way and thus avoid
			// flakes. More specifically, we were observing the test failing to get objects from the API server during move,
			// so we are now testing the API servers are stable before starting move.
			Consistently(func() error {
				kubeSystem := &corev1.Namespace{}
				return input.BootstrapClusterProxy.GetClient().
					Get(ctx, client.ObjectKey{Name: "kube-system"}, kubeSystem)
			}, "5s", "100ms").Should(BeNil(), "Failed to assert bootstrap API server stability")
			Consistently(func() error {
				kubeSystem := &corev1.Namespace{}
				return selfHostedClusterProxy.GetClient().
					Get(ctx, client.ObjectKey{Name: "kube-system"}, kubeSystem)
			}, "5s", "100ms").Should(BeNil(), "Failed to assert self-hosted API server stability")

			By("Moving the cluster back to bootstrap")
			clusterctl.Move(ctx, clusterctl.MoveInput{
				LogFolder: filepath.Join(
					input.ArtifactFolder,
					"clusters",
					clusterResources.Cluster.Name,
				),
				ClusterctlConfigPath: input.ClusterctlConfigPath,
				FromKubeconfigPath:   selfHostedClusterProxy.GetKubeconfigPath(),
				ToKubeconfigPath:     input.BootstrapClusterProxy.GetKubeconfigPath(),
				Namespace:            selfHostedNamespace.Name,
			})

			By("Waiting for the cluster to be reconciled after moving back to bootstrap")
			clusterResources.Cluster = framework.DiscoveryAndWaitForCluster(
				ctx,
				framework.DiscoveryAndWaitForClusterInput{
					Getter:    input.BootstrapClusterProxy.GetClient(),
					Namespace: namespace.Name,
					Name:      clusterResources.Cluster.Name,
				},
				input.E2EConfig.GetIntervals(specName, "wait-cluster")...)
		}
		if selfHostedCancelWatches != nil {
			selfHostedCancelWatches()
		}

		// Dumps all the resources in the spec namespace, then cleanups the cluster object and the spec namespace itself.
		dumpSpecResourcesAndCleanup(
			ctx,
			specName,
			input.BootstrapClusterProxy,
			input.ArtifactFolder,
			namespace,
			cancelWatches,
			clusterResources.Cluster,
			input.E2EConfig.GetIntervals,
			input.SkipCleanup,
		)
	})
}

func hasProvider(ctx context.Context, c client.Client, providerName string) bool {
	providerList := clusterctlv1.ProviderList{}
	Eventually(func() error {
		return c.List(ctx, &providerList)
	}, "1m", "5s").Should(Succeed(), "Failed to list the Providers")

	return slices.ContainsFunc(providerList.Items, func(provider clusterctlv1.Provider) bool {
		return provider.GetName() == providerName
	})
}

func setupSpecNamespace(
	ctx context.Context,
	specName string,
	clusterProxy framework.ClusterProxy,
	artifactFolder string,
) (*corev1.Namespace, context.CancelFunc) {
	capie2e.Byf("Creating a namespace for hosting the %q test spec", specName)
	namespace, cancelWatches := framework.CreateNamespaceAndWatchEvents(
		ctx,
		framework.CreateNamespaceAndWatchEventsInput{
			Creator:   clusterProxy.GetClient(),
			ClientSet: clusterProxy.GetClientSet(),
			Name:      fmt.Sprintf("%s-%s", specName, util.RandomString(6)),
			LogFolder: filepath.Join(artifactFolder, "clusters", clusterProxy.GetName()),
		},
	)

	return namespace, cancelWatches
}

// dumpAllResources dumps all the resources in the spec namespace and the workload cluster.
func dumpAllResources(
	ctx context.Context,
	clusterProxy framework.ClusterProxy,
	artifactFolder string,
	namespace *corev1.Namespace,
	cluster *clusterv1.Cluster,
) {
	capie2e.Byf("Dumping logs from the %q workload cluster", cluster.Name)

	// Dump all the logs from the workload cluster.
	clusterProxy.CollectWorkloadClusterLogs(
		ctx,
		cluster.Namespace,
		cluster.Name,
		filepath.Join(artifactFolder, "clusters", cluster.Name),
	)

	capie2e.Byf("Dumping all the Cluster API resources in the %q namespace", namespace.Name)

	// Dump all Cluster API related resources to artifacts.
	framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
		Lister:    clusterProxy.GetClient(),
		Namespace: namespace.Name,
		LogPath:   filepath.Join(artifactFolder, "clusters", clusterProxy.GetName(), "resources"),
	})

	// If the cluster still exists, dump pods and nodes of the workload cluster.
	if err := clusterProxy.GetClient().Get(ctx, client.ObjectKeyFromObject(cluster), &clusterv1.Cluster{}); err == nil {
		capie2e.Byf("Dumping Pods and Nodes of Cluster %s", klog.KObj(cluster))
		framework.DumpResourcesForCluster(ctx, framework.DumpResourcesForClusterInput{
			Lister: clusterProxy.GetWorkloadCluster(ctx, cluster.Namespace, cluster.Name).
				GetClient(),
			Cluster: cluster,
			LogPath: filepath.Join(artifactFolder, "clusters", cluster.Name, "resources"),
			Resources: []framework.DumpNamespaceAndGVK{
				{
					GVK: schema.GroupVersionKind{
						Version: corev1.SchemeGroupVersion.Version,
						Kind:    "Pod",
					},
				},
				{
					GVK: schema.GroupVersionKind{
						Version: corev1.SchemeGroupVersion.Version,
						Kind:    "Node",
					},
				},
			},
		})
	}
}

// dumpSpecResourcesAndCleanup dumps all the resources in the spec namespace and cleans up the spec namespace.
func dumpSpecResourcesAndCleanup(
	ctx context.Context,
	specName string,
	clusterProxy framework.ClusterProxy,
	artifactFolder string,
	namespace *corev1.Namespace,
	cancelWatches context.CancelFunc,
	cluster *clusterv1.Cluster,
	intervalsGetter func(spec, key string) []interface{},
	skipCleanup bool,
) {
	// Dump all the resources in the spec namespace and the workload cluster.
	dumpAllResources(ctx, clusterProxy, artifactFolder, namespace, cluster)

	if !skipCleanup {
		capie2e.Byf("Deleting cluster %s", klog.KObj(cluster))
		// While https://github.com/kubernetes-sigs/cluster-api/issues/2955 is addressed in future iterations, there is a
		// chance that cluster variable is not set even if the cluster exists, so we are calling DeleteAllClustersAndWait
		// instead of DeleteClusterAndWait
		framework.DeleteAllClustersAndWait(ctx, framework.DeleteAllClustersAndWaitInput{
			Client:    clusterProxy.GetClient(),
			Namespace: namespace.Name,
		}, intervalsGetter(specName, "wait-delete-cluster")...)

		capie2e.Byf("Deleting namespace used for hosting the %q test spec", specName)
		framework.DeleteNamespace(ctx, framework.DeleteNamespaceInput{
			Deleter: clusterProxy.GetClient(),
			Name:    namespace.Name,
		})
	}
	cancelWatches()
}
