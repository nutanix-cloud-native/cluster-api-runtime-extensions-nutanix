//go:build e2e

// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	capie2eframework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
)

// QuickStartSpec wraps the Cluster API quick-start e2e workflow with nil-safe AfterEach cleanup
// so failures or Skips in BeforeEach (such as infrastructure IP reservation timeouts) do not
// panic dereferencing uninitialized spec state.
func QuickStartSpec(ctx context.Context, inputGetter func() capie2e.QuickStartSpecInput) {
	var (
		specName              = "quick-start"
		input                 capie2e.QuickStartSpecInput
		namespace             *corev1.Namespace
		clusterClassNamespace *corev1.Namespace
		cancelWatches         context.CancelFunc
		clusterResources      *clusterctl.ApplyClusterTemplateAndWaitResult
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
		Expect(input.E2EConfig.Variables).To(HaveKey(capie2e.KubernetesVersion))
		Expect(input.ExtensionServiceNamespace).To(
			BeEmpty(),
			"ExtensionServiceNamespace is not supported by QuickStartSpec",
		)
		Expect(input.ExtensionServiceName).To(
			BeEmpty(),
			"ExtensionServiceName is not supported by QuickStartSpec",
		)

		namespace, cancelWatches = capie2eframework.SetupSpecNamespace(
			ctx,
			specName,
			input.BootstrapClusterProxy,
			input.ArtifactFolder,
			input.PostNamespaceCreated,
		)

		if input.DeployClusterClassInSeparateNamespace {
			clusterClassNamespace = capie2eframework.CreateNamespace(
				ctx,
				capie2eframework.CreateNamespaceInput{
					Creator: input.BootstrapClusterProxy.GetClient(),
					Name:    fmt.Sprintf("%s-clusterclass", namespace.Name),
				},
				"40s",
				"10s",
			)
			Expect(clusterClassNamespace).ToNot(BeNil(), "Failed to create namespace")
		}

		clusterResources = new(clusterctl.ApplyClusterTemplateAndWaitResult)
	})

	It("Should create a workload cluster", func() {
		By("Creating a workload cluster")

		infrastructureProvider := clusterctl.DefaultInfrastructureProvider
		if input.InfrastructureProvider != nil {
			infrastructureProvider = *input.InfrastructureProvider
		}

		flavor := clusterctl.DefaultFlavor
		if input.Flavor != nil {
			flavor = *input.Flavor
		}

		controlPlaneMachineCount := ptr.To[int64](1)
		if input.ControlPlaneMachineCount != nil {
			controlPlaneMachineCount = input.ControlPlaneMachineCount
		}

		workerMachineCount := ptr.To[int64](1)
		if input.WorkerMachineCount != nil {
			workerMachineCount = input.WorkerMachineCount
		}

		clusterName := fmt.Sprintf("%s-%s", specName, util.RandomString(6))
		if input.ClusterName != nil {
			clusterName = *input.ClusterName
		}

		variables := map[string]string{
			"EXTENSION_CONFIG_NAME": input.ExtensionConfigName,
		}
		maps.Copy(variables, input.ClusterctlVariables)

		if input.DeployClusterClassInSeparateNamespace {
			variables["CLUSTER_CLASS_NAMESPACE"] = clusterClassNamespace.Name
			By("Creating a cluster referencing a ClusterClass from another namespace")
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
				ClusterctlVariables:      variables,
				KubeconfigPath:           input.BootstrapClusterProxy.GetKubeconfigPath(),
				InfrastructureProvider:   infrastructureProvider,
				Flavor:                   flavor,
				Namespace:                namespace.Name,
				ClusterName:              clusterName,
				KubernetesVersion:        input.E2EConfig.MustGetVariable(capie2e.KubernetesVersion),
				ControlPlaneMachineCount: controlPlaneMachineCount,
				WorkerMachineCount:       workerMachineCount,
			},
			ControlPlaneWaiters:          input.ControlPlaneWaiters,
			WaitForClusterIntervals:      input.E2EConfig.GetIntervals(specName, "wait-cluster"),
			WaitForControlPlaneIntervals: input.E2EConfig.GetIntervals(specName, "wait-control-plane"),
			WaitForMachineDeployments:    input.E2EConfig.GetIntervals(specName, "wait-worker-nodes"),
			PostMachinesProvisioned: func() {
				if input.PostMachinesProvisioned != nil {
					input.PostMachinesProvisioned(
						input.BootstrapClusterProxy,
						namespace.Name,
						clusterName,
					)
				}
			},
		}, clusterResources)

		capie2e.Byf("Verify Cluster Available condition is true")
		capie2eframework.VerifyClusterAvailable(ctx, capie2eframework.VerifyClusterAvailableInput{
			Getter:    input.BootstrapClusterProxy.GetClient(),
			Name:      clusterResources.Cluster.Name,
			Namespace: clusterResources.Cluster.Namespace,
		})

		capie2e.Byf("Verify Machines Ready condition is true")
		capie2eframework.VerifyMachinesReady(ctx, capie2eframework.VerifyMachinesReadyInput{
			Lister:    input.BootstrapClusterProxy.GetClient(),
			Name:      clusterResources.Cluster.Name,
			Namespace: clusterResources.Cluster.Namespace,
		})

		By("PASSED!")
	})

	AfterEach(func() {
		CleanupQuickStartSpec(
			ctx,
			specName,
			&input,
			namespace,
			clusterClassNamespace,
			cancelWatches,
			clusterResources,
		)
	})
}

// CleanupQuickStartSpec dumps spec resources and deletes the test namespaces created by QuickStartSpec.
func CleanupQuickStartSpec(
	ctx context.Context,
	specName string,
	input *capie2e.QuickStartSpecInput,
	namespace *corev1.Namespace,
	clusterClassNamespace *corev1.Namespace,
	cancelWatches context.CancelFunc,
	clusterResources *clusterctl.ApplyClusterTemplateAndWaitResult,
) {
	if input == nil {
		if cancelWatches != nil {
			cancelWatches()
		}
		return
	}

	dumpSpecResourcesAndCleanup(
		ctx,
		specName,
		input.BootstrapClusterProxy,
		input.ClusterctlConfigPath,
		input.ArtifactFolder,
		namespace,
		cancelWatches,
		clusterFromResult(clusterResources),
		intervalsGetterFromConfig(input.E2EConfig),
		input.SkipCleanup,
	)
	if !input.SkipCleanup &&
		input.DeployClusterClassInSeparateNamespace &&
		clusterClassNamespace != nil &&
		input.BootstrapClusterProxy != nil {
		specByf("Deleting namespace used for hosting the %q test spec ClusterClass", specName)
		capie2eframework.DeleteNamespace(ctx, capie2eframework.DeleteNamespaceInput{
			Deleter: input.BootstrapClusterProxy.GetClient(),
			Name:    clusterClassNamespace.Name,
		})
	}
}
