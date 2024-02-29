//go:build e2e

// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	addonsv1 "sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	capibootstrap "sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/d2iq-labs/capi-runtime-extensions/test/framework/bootstrap"
	clusterctltemp "github.com/d2iq-labs/capi-runtime-extensions/test/framework/clusterctl"
)

func init() { //nolint:gochecknoinits // Idiomatically used to set up flags.
	flag.StringVar(&configPath, "e2e.config", "", "path to the e2e config file")
	flag.StringVar(
		&artifactFolder,
		"e2e.artifacts-folder",
		"",
		"folder where e2e test artifact should be stored",
	)
	flag.BoolVar(
		&skipCleanup,
		"e2e.skip-resource-cleanup",
		false,
		"if true, the resource cleanup after tests will be skipped",
	)
	flag.BoolVar(
		&skipLogCollection,
		"e2e.skip-log-collection",
		false,
		"if true, the log collection after tests will be skipped",
	)
	flag.BoolVar(
		&useExistingCluster,
		"e2e.use-existing-cluster",
		false,
		"if true, the test uses the current cluster instead of creating a new one (default discovery rules apply)",
	)
	flag.StringVar(
		&bootstrapNodeImageRepository,
		"e2e.bootstrap-kind-image",
		"ghcr.io/mesosphere/kind-node",
		"the image to use for the bootstrap cluster",
	)
	flag.StringVar(
		&bootstrapKubernetesVersion,
		"e2e.bootstrap-kind-version",
		"v1.29.2",
		"the version of the image used in bootstrap cluster",
	)
}

func TestE2E(t *testing.T) {
	ctrl.SetLogger(klog.Background())
	RegisterFailHandler(Fail)
	RunSpecs(t, "cre-e2e")
}

// Using a SynchronizedBeforeSuite for controlling how to create resources shared across ParallelNodes (~ginkgo
// threads). The local clusterctl repository & the bootstrap cluster are created once and shared across all the tests.
var _ = SynchronizedBeforeSuite(func() []byte {
	Expect(
		configPath,
	).To(BeAnExistingFile(), "Invalid test suite argument. e2e.config should be an existing file.")
	Expect(
		os.MkdirAll(artifactFolder, 0o755),
	).To(Succeed(), "Invalid test suite argument. Can't create e2e.artifacts-folder %q", artifactFolder)

	By("Initializing a runtime.Scheme with all the GVK relevant for this test")
	scheme := initScheme()

	capie2e.Byf("Loading the e2e test configuration from %q", configPath)
	e2eConfig = loadE2EConfig(configPath)

	capie2e.Byf("Creating a clusterctl local repository into %q", artifactFolder)
	clusterctlConfigPath = createClusterctlLocalRepository(
		e2eConfig,
		filepath.Join(artifactFolder, "repository"),
	)

	By("Setting up the bootstrap cluster")
	bootstrapClusterProvider, bootstrapClusterProxy = setupBootstrapCluster(
		e2eConfig,
		scheme,
		useExistingCluster,
	)

	By("Initializing the bootstrap cluster")
	initBootstrapCluster(bootstrapClusterProxy, e2eConfig, clusterctlConfigPath, artifactFolder)

	// encode the e2e config into the byte array.
	var configBuf bytes.Buffer
	enc := gob.NewEncoder(&configBuf)
	Expect(enc.Encode(e2eConfig)).To(Succeed())
	configStr := base64.StdEncoding.EncodeToString(configBuf.Bytes())

	return []byte(
		strings.Join([]string{
			artifactFolder,
			clusterctlConfigPath,
			configStr,
			bootstrapClusterProxy.GetKubeconfigPath(),
		}, ","),
	)
}, func(data []byte) {
	// Before each ParallelNode.

	parts := strings.Split(string(data), ",")
	Expect(parts).To(HaveLen(4))

	artifactFolder = parts[0]
	clusterctlConfigPath = parts[1]

	// Decode the e2e config
	configBytes, err := base64.StdEncoding.DecodeString(parts[2])
	Expect(err).NotTo(HaveOccurred())
	buf := bytes.NewBuffer(configBytes)
	dec := gob.NewDecoder(buf)
	Expect(dec.Decode(&e2eConfig)).To(Succeed())

	// we unset Kubernetes version variables to make sure we use the ones resolved from the first Ginkgo ParallelNode in
	// the e2e config.
	os.Unsetenv(capie2e.KubernetesVersion)
	os.Unsetenv(capie2e.KubernetesVersionUpgradeFrom)
	os.Unsetenv(capie2e.KubernetesVersionUpgradeTo)

	kubeconfigPath := parts[3]
	bootstrapClusterProxy = framework.NewClusterProxy(
		"bootstrap",
		kubeconfigPath,
		initScheme(),
		framework.WithMachineLogCollector(framework.DockerLogCollector{}),
	)
})

// Using a SynchronizedAfterSuite for controlling how to delete resources shared across ParallelNodes (~ginkgo threads).
// The bootstrap cluster is shared across all the tests, so it should be deleted only after all ParallelNodes completes.
// The local clusterctl repository is preserved like everything else created into the artifact folder.
var _ = SynchronizedAfterSuite(func() {
	// After each ParallelNode.
}, func() {
	// After all ParallelNodes.

	By("Tearing down the management cluster")
	if !skipCleanup {
		tearDown(bootstrapClusterProvider, bootstrapClusterProxy)
	}
})

func loadE2EConfig(configPath string) *clusterctl.E2EConfig {
	config := clusterctltemp.LoadE2EConfig(
		context.TODO(),
		clusterctl.LoadE2EConfigInput{ConfigPath: configPath},
	)
	Expect(config).NotTo(BeNil(), "Failed to load E2E config from %s", configPath)

	config.Providers = slices.DeleteFunc(config.Providers, func(p clusterctl.ProviderConfig) bool {
		switch p.Name {
		case "aws":
			_, found := os.LookupEnv("AWS_B64ENCODED_CREDENTIALS")
			return !found
		default:
			return false
		}
	})

	return config
}

func createClusterctlLocalRepository(config *clusterctl.E2EConfig, repositoryFolder string) string {
	createRepositoryInput := clusterctl.CreateRepositoryInput{
		E2EConfig:        config,
		RepositoryFolder: repositoryFolder,
	}

	clusterctlConfig := clusterctl.CreateRepository(context.TODO(), createRepositoryInput)
	Expect(
		clusterctlConfig,
	).To(BeAnExistingFile(), "The clusterctl config file does not exists in the local repository %s", repositoryFolder)
	return clusterctlConfig
}

func initScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	framework.TryAddDefaultSchemes(scheme)
	Expect(addonsv1.AddToScheme(scheme)).To(Succeed())
	return scheme
}

func setupBootstrapCluster(
	config *clusterctl.E2EConfig,
	scheme *runtime.Scheme,
	useExistingCluster bool,
) (capibootstrap.ClusterProvider, framework.ClusterProxy) {
	var clusterProvider capibootstrap.ClusterProvider
	kubeconfigPath := ""
	if !useExistingCluster {
		clusterProvider = bootstrap.CreateKindBootstrapClusterAndLoadImages(
			context.TODO(),
			bootstrap.CreateKindBootstrapClusterAndLoadImagesInput{
				NodeImageRepository: bootstrapNodeImageRepository,
				CreateKindBootstrapClusterAndLoadImagesInput: capibootstrap.CreateKindBootstrapClusterAndLoadImagesInput{
					Name:               config.ManagementClusterName,
					RequiresDockerSock: config.HasDockerProvider(),
					Images:             config.Images,
					KubernetesVersion:  bootstrapKubernetesVersion,
				},
			},
		)
		Expect(clusterProvider).NotTo(BeNil(), "Failed to create a bootstrap cluster")

		kubeconfigPath = clusterProvider.GetKubeconfigPath()
		Expect(
			kubeconfigPath,
		).To(BeAnExistingFile(), "Failed to get the kubeconfig file for the bootstrap cluster")
	} else {
		// Loading image for already created cluster
		imagesInput := capibootstrap.LoadImagesToKindClusterInput{
			Name:   "cre-e2e",
			Images: config.Images,
		}
		err := capibootstrap.LoadImagesToKindCluster(context.TODO(), imagesInput)
		Expect(err).To(BeNil(), "Failed to load images to the bootstrap cluster: %s", err)
	}

	clusterProxy := framework.NewClusterProxy("bootstrap", kubeconfigPath, scheme)
	Expect(clusterProxy).NotTo(BeNil(), "Failed to get a bootstrap cluster proxy")
	return clusterProvider, clusterProxy
}

func initBootstrapCluster(
	bootstrapClusterProxy framework.ClusterProxy,
	config *clusterctl.E2EConfig,
	clusterctlConfig, artifactFolder string,
) {
	clusterctl.InitManagementClusterAndWatchControllerLogs(
		context.TODO(),
		clusterctl.InitManagementClusterAndWatchControllerLogsInput{
			ClusterProxy:              bootstrapClusterProxy,
			ClusterctlConfigPath:      clusterctlConfig,
			InfrastructureProviders:   config.InfrastructureProviders(),
			AddonProviders:            config.AddonProviders(),
			RuntimeExtensionProviders: config.RuntimeExtensionProviders(),
			LogFolder: filepath.Join(
				artifactFolder,
				"clusters",
				bootstrapClusterProxy.GetName(),
			),
		},
		config.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)
}

func tearDown(
	bootstrapClusterProvider capibootstrap.ClusterProvider,
	bootstrapClusterProxy framework.ClusterProxy,
) {
	if bootstrapClusterProxy != nil {
		bootstrapClusterProxy.Dispose(context.TODO())
	}
	if bootstrapClusterProvider != nil {
		bootstrapClusterProvider.Dispose(context.TODO())
	}
}
