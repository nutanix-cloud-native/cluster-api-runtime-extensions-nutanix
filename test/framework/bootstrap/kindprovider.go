//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	kindv1 "sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	kind "sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"
	kindexec "sigs.k8s.io/kind/pkg/exec"
)

const (
	// defaultDockerSocketPath is the canonical Docker socket location that kind and CAPD expect
	// inside the bootstrap cluster node.
	defaultDockerSocketPath = "/var/run/docker.sock"

	// DockerSocketEnvVar overrides the host Docker socket mounted into the bootstrap cluster.
	// This is required for rootless Docker, where the socket lives under $XDG_RUNTIME_DIR
	// (e.g. /run/user/1000/docker.sock) rather than /var/run/docker.sock.
	DockerSocketEnvVar = "CAREN_E2E_DOCKER_SOCKET"
)

// CreateKindBootstrapClusterAndLoadImagesInput is the input for CreateKindBootstrapClusterAndLoadImages.
type CreateKindBootstrapClusterAndLoadImagesInput struct {
	// NodeImageRepository defines the image repository to use for the bootstrap cluster nodes.
	NodeImageRepository string

	bootstrap.CreateKindBootstrapClusterAndLoadImagesInput
}

// CreateKindBootstrapClusterAndLoadImages returns a new Kubernetes cluster with pre-loaded images.
func CreateKindBootstrapClusterAndLoadImages(
	ctx context.Context,
	input CreateKindBootstrapClusterAndLoadImagesInput, //nolint:gocritic // Copied from upstream.
) bootstrap.ClusterProvider {
	Expect(ctx).NotTo(BeNil(), "ctx is required for CreateKindBootstrapClusterAndLoadImages")
	Expect(
		input.Name,
	).ToNot(BeEmpty(), "Invalid argument. Name can't be empty when calling CreateKindBootstrapClusterAndLoadImages")

	fmt.Fprintf(GinkgoWriter, "INFO: Creating a kind cluster with name %q\n", input.Name)

	nodeImageRepository := bootstrap.DefaultNodeImageRepository
	if input.NodeImageRepository != "" {
		nodeImageRepository = input.NodeImageRepository
	}
	nodeImage := ""
	if input.KubernetesVersion != "" {
		nodeImage = fmt.Sprintf("%s:%s", nodeImageRepository, input.KubernetesVersion)
	}

	clusterProvider := newClusterProvider(input, nodeImage)
	Expect(clusterProvider).ToNot(BeNil(), "Failed to create a kind cluster")

	clusterProvider.Create(ctx)
	Expect(
		clusterProvider.GetKubeconfigPath(),
	).To(
		BeAnExistingFile(),
		"The kubeconfig file for the kind cluster with name %q does not exists at %q as expected",
		input.Name,
		clusterProvider.GetKubeconfigPath(),
	)

	fmt.Fprintf(
		GinkgoWriter,
		"INFO: The kubeconfig file for the kind cluster is %s\n",
		clusterProvider.GetKubeconfigPath(),
	)

	err := bootstrap.LoadImagesToKindCluster(ctx, bootstrap.LoadImagesToKindClusterInput{
		Name:   input.Name,
		Images: input.Images,
	})
	if err != nil {
		clusterProvider.Dispose(ctx)
		Expect(err).ToNot(HaveOccurred()) // re-surface the error to fail the test
	}

	return clusterProvider
}

// newClusterProvider returns a bootstrap.ClusterProvider. When the Docker socket is required and
// a non-default host socket path is configured (rootless Docker), it returns a CAREN provider that
// mounts that socket; otherwise it returns the upstream provider unchanged.
func newClusterProvider(
	input CreateKindBootstrapClusterAndLoadImagesInput, //nolint:gocritic // Mirrors caller.
	nodeImage string,
) bootstrap.ClusterProvider {
	dockerSocketPath := dockerSocketHostPath()
	if input.RequiresDockerSock && dockerSocketPath != defaultDockerSocketPath {
		fmt.Fprintf(
			GinkgoWriter,
			"INFO: Mounting host Docker socket %q into the bootstrap cluster\n",
			dockerSocketPath,
		)
		return &kindClusterProvider{
			name:             input.Name,
			nodeImage:        nodeImage,
			dockerSocketPath: dockerSocketPath,
			ipFamily:         input.IPFamily,
			logFolder:        input.LogFolder,
		}
	}

	options := []bootstrap.KindClusterOption{}
	if nodeImage != "" {
		options = append(options, bootstrap.WithNodeImage(nodeImage))
	}
	if input.RequiresDockerSock {
		options = append(options, bootstrap.WithDockerSockMount())
	}
	if input.IPFamily == "IPv6" {
		options = append(options, bootstrap.WithIPv6Family())
	}
	if input.IPFamily == "dual" {
		options = append(options, bootstrap.WithDualStackFamily())
	}
	if input.LogFolder != "" {
		options = append(options, bootstrap.LogFolder(input.LogFolder))
	}
	return bootstrap.NewKindClusterProvider(input.Name, options...)
}

// dockerSocketHostPath resolves the host path of the Docker socket to mount into the bootstrap
// cluster. It honours CAREN_E2E_DOCKER_SOCKET first, then a unix:// DOCKER_HOST (the standard
// rootless Docker setup), and finally falls back to the canonical /var/run/docker.sock.
func dockerSocketHostPath() string {
	if socket := strings.TrimSpace(os.Getenv(DockerSocketEnvVar)); socket != "" {
		return socket
	}
	if dockerHost := strings.TrimSpace(os.Getenv("DOCKER_HOST")); strings.HasPrefix(
		dockerHost, "unix://",
	) {
		return strings.TrimPrefix(dockerHost, "unix://")
	}
	return defaultDockerSocketPath
}

// kindClusterProvider creates a kind cluster that mounts a configurable host Docker socket at the
// canonical /var/run/docker.sock inside the node. It mirrors the upstream provider but adds the
// configurable host socket path, which upstream hardcodes. It implements bootstrap.ClusterProvider.
type kindClusterProvider struct {
	name             string
	nodeImage        string
	dockerSocketPath string
	ipFamily         string
	logFolder        string
	kubeconfigPath   string
}

// Create a Kubernetes cluster using kind.
func (k *kindClusterProvider) Create(ctx context.Context) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for Create")

	f, err := os.CreateTemp("", "e2e-kind")
	Expect(err).ToNot(HaveOccurred(), "Failed to create kubeconfig file for the kind cluster %q", k.name)
	k.kubeconfigPath = f.Name()

	cfg := &kindv1.Cluster{
		Nodes: []kindv1.Node{{Role: kindv1.ControlPlaneRole}},
	}
	switch k.ipFamily {
	case "IPv6":
		cfg.Networking.IPFamily = kindv1.IPv6Family
	case "dual":
		cfg.Networking.IPFamily = kindv1.DualStackFamily
	}
	kindv1.SetDefaultsCluster(cfg)

	cfg.Nodes[0].ExtraMounts = append(cfg.Nodes[0].ExtraMounts, kindv1.Mount{
		HostPath:      k.dockerSocketPath,
		ContainerPath: defaultDockerSocketPath,
	})

	nodeImage := fmt.Sprintf(
		"%s:%s", bootstrap.DefaultNodeImageRepository, bootstrap.DefaultNodeImageVersion,
	)
	if k.nodeImage != "" {
		nodeImage = k.nodeImage
	}

	provider := kind.NewProvider(kind.ProviderWithLogger(cmd.NewLogger()))
	err = provider.Create(
		k.name,
		kind.CreateWithKubeconfigPath(k.kubeconfigPath),
		kind.CreateWithV1Alpha4Config(cfg),
		kind.CreateWithNodeImage(nodeImage),
		kind.CreateWithRetain(true),
	)
	if err != nil {
		if k.logFolder != "" {
			if logErr := provider.CollectLogs(k.name, k.logFolder); logErr != nil {
				fmt.Fprintf(GinkgoWriter, "Failed to collect logs from kind: %v\n", logErr)
			}
		}
		errStr := fmt.Sprintf("Failed to create kind cluster %q: %v", k.name, err)
		var runErr *kindexec.RunError
		if errors.As(err, &runErr) {
			errStr += "\n" + string(runErr.Output)
		}
		Expect(err).ToNot(HaveOccurred(), errStr)
	}
}

// GetKubeconfigPath returns the path to the kubeconfig file for the cluster.
func (k *kindClusterProvider) GetKubeconfigPath() string {
	return k.kubeconfigPath
}

// Dispose the kind cluster and its kubeconfig file.
func (k *kindClusterProvider) Dispose(ctx context.Context) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for Dispose")

	if err := kind.NewProvider().Delete(k.name, k.kubeconfigPath); err != nil {
		fmt.Fprintf(
			GinkgoWriter,
			"Deleting the kind cluster %q failed. You may need to remove this by hand.\n",
			k.name,
		)
	}
	if err := os.Remove(k.kubeconfigPath); err != nil {
		fmt.Fprintf(
			GinkgoWriter,
			"Deleting the kubeconfig file %q failed. You may need to remove this by hand.\n",
			k.kubeconfigPath,
		)
	}
}
