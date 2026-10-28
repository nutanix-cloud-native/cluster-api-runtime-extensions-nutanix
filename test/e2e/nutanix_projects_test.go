//go:build e2e

// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	capiframework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
)

// The Nutanix projects suite deploys a cluster into a non-default Prism Central project and
// exercises CAPX, CCM and CSI together against it, so that a regression in any one of them - for
// example when a provider version is bumped - is caught here. Each scenario runs twice: once with
// the broad-scope Prism Central user CI normally uses, and once with a user scoped to that project
// only, because project-scoped credentials see a different, more restricted view of Prism Central.
const (
	// nutanixProjectFlavor deploys into NUTANIX_PROJECT_NAME with broad-scope credentials.
	nutanixProjectFlavor = "topology-cilium-helm-addon-with-project"
	// nutanixProjectScopedUserFlavor deploys the same cluster, but sources the CAPX/CCM, CSI and
	// konnector agent credentials from NUTANIX_PROJECT_SCOPE_USER.
	nutanixProjectScopedUserFlavor = "topology-cilium-helm-addon-with-project-scoped-user"
)

var _ = Describe("Nutanix projects", Label("provider:Nutanix", "projects"), func() {
	for _, scenario := range []struct {
		name                      string
		label                     string
		flavor                    string
		clusterNamePrefix         string
		requiresProjectScopedUser bool
	}{
		{
			name:              "as a broad-scope user",
			label:             "broad-scope-user",
			flavor:            nutanixProjectFlavor,
			clusterNamePrefix: "project-",
		},
		{
			name:                      "as a project-scoped user",
			label:                     "project-scope-user",
			flavor:                    nutanixProjectScopedUserFlavor,
			clusterNamePrefix:         "project-scoped-",
			requiresProjectScopedUser: true,
		},
	} {
		Context(scenario.name, Label(scenario.label), func() {
			var (
				testE2EConfig                    *clusterctl.E2EConfig
				clusterLocalClusterctlConfigPath string
			)

			BeforeEach(func() {
				skipUnlessNutanixProjectsConfigured()
				if scenario.requiresProjectScopedUser {
					skipUnlessProjectScopedUserConfigured()
				}

				testE2EConfig = e2eConfig.DeepCopy()
				applyProviderKubernetesVersionOverride(testE2EConfig, "nutanix")
				reserveNutanixIPsForCluster(testE2EConfig)

				clusterLocalTempDir, err := os.MkdirTemp("", "clusterctl-nutanix-projects-")
				Expect(err).ToNot(HaveOccurred())
				DeferCleanup(func() {
					Expect(os.RemoveAll(clusterLocalTempDir)).To(Succeed())
				})

				clusterLocalClusterctlConfigPath = createClusterctlLocalRepository(
					testE2EConfig,
					clusterLocalTempDir,
				)
			})

			capie2e.QuickStartSpec(ctx, func() capie2e.QuickStartSpecInput {
				const maxClusterNameLength = 63

				return capie2e.QuickStartSpecInput{
					ClusterName: ptr.To(scenario.clusterNamePrefix +
						util.RandomString(maxClusterNameLength-len(scenario.clusterNamePrefix))),
					E2EConfig:              testE2EConfig,
					ClusterctlConfigPath:   clusterLocalClusterctlConfigPath,
					BootstrapClusterProxy:  bootstrapClusterProxy,
					ArtifactFolder:         artifactFolder,
					SkipCleanup:            skipCleanup,
					Flavor:                 ptr.To(scenario.flavor),
					InfrastructureProvider: ptr.To("nutanix"),
					PostMachinesProvisioned: func(proxy capiframework.ClusterProxy, namespace, clusterName string) {
						workloadCluster := VerifyProvisionedCluster(
							ctx,
							VerifyProvisionedClusterInput{
								ClusterProxy: proxy,
								Namespace:    namespace,
								ClusterName:  clusterName,
								Provider:     nutanixProvider,
								Flavor:       scenario.flavor,
								E2EConfig:    testE2EConfig,
							},
						)

						AssertClusterPinnedToSingleNutanixProject(workloadCluster)
						AssertNodesInitializedByNutanixCCM(
							ctx,
							proxy,
							workloadCluster,
							testE2EConfig.GetIntervals(scenario.flavor, "wait-nodes-ready"),
						)
						AssertNutanixCSIDynamicProvisioning(
							ctx,
							proxy,
							workloadCluster,
							testE2EConfig.GetIntervals(scenario.flavor, "wait-statefulset"),
						)
					},
				}
			})
		})
	}

	Context("as a project-scoped user, in a project it cannot access", Label("project-scope-user"), func() {
		var (
			testE2EConfig                    *clusterctl.E2EConfig
			clusterLocalClusterctlConfigPath string
			inaccessibleProjectName          string
		)

		BeforeEach(func() {
			skipUnlessNutanixProjectsConfigured()
			skipUnlessProjectScopedUserConfigured()

			inaccessibleProjectName = strings.TrimSpace(
				e2eConfig.GetVariableOrEmpty(nutanixInaccessibleProjectNameVar),
			)
			if inaccessibleProjectName == "" {
				Skip(nutanixInaccessibleProjectNameVar + " is required for the inaccessible project scenario")
			}

			testE2EConfig = e2eConfig.DeepCopy()
			applyProviderKubernetesVersionOverride(testE2EConfig, "nutanix")
			// The cluster is never provisioned, but the template still needs both endpoint
			// addresses to render.
			reserveNutanixIPsForCluster(testE2EConfig)

			clusterLocalTempDir, err := os.MkdirTemp("", "clusterctl-nutanix-projects-denied-")
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(func() {
				Expect(os.RemoveAll(clusterLocalTempDir)).To(Succeed())
			})

			clusterLocalClusterctlConfigPath = createClusterctlLocalRepository(
				testE2EConfig,
				clusterLocalTempDir,
			)
		})

		It("Should reject the cluster", func() {
			By("Creating a cluster pointed at a project the project-scoped user is not granted on")
			err := applyClusterFromFlavor(applyClusterFromFlavorInput{
				testE2EConfig:        testE2EConfig,
				clusterctlConfigPath: clusterLocalClusterctlConfigPath,
				flavor:               nutanixProjectScopedUserFlavor,
				clusterName:          "project-denied-" + util.RandomString(6),
				projectNameOverride:  inaccessibleProjectName,
			})

			Expect(err).To(HaveOccurred(),
				"CAREN should reject a cluster in a project the credentials cannot resolve",
			)
			Expect(err.Error()).To(MatchRegexp(
				`(?i)(failed to resolve project|project.*(not found|not accessible))`,
			), "rejection should name the unresolvable project as the cause")
		})
	})
})
