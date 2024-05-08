//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
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

	options := []bootstrap.KindClusterOption{}
	nodeImageRepository := bootstrap.DefaultNodeImageRepository
	if input.NodeImageRepository != "" {
		nodeImageRepository = input.NodeImageRepository
	}

	if input.KubernetesVersion != "" {
		options = append(
			options,
			bootstrap.WithNodeImage(
				fmt.Sprintf("%s:%s", nodeImageRepository, input.KubernetesVersion),
			),
		)
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
	clusterProvider := bootstrap.NewKindClusterProvider(input.Name, options...)
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
