# Changelog

## 0.3.5 (2024-02-08)

<!-- Release notes generated using configuration in .github/release.yaml at main -->



**Full Changelog**: https://github.com/d2iq-labs/capi-runtime-extensions/compare/v0.3.4...v0.3.5

## 0.3.4 (2024-02-08)

<!-- Release notes generated using configuration in .github/release.yaml at main -->



**Full Changelog**: https://github.com/d2iq-labs/capi-runtime-extensions/compare/v0.3.3...v0.3.4

## 0.3.3 (2024-02-08)

<!-- Release notes generated using configuration in .github/release.yaml at main -->



**Full Changelog**: https://github.com/d2iq-labs/capi-runtime-extensions/compare/v0.3.2...v0.3.3

## 0.3.2 (2024-02-08)

<!-- Release notes generated using configuration in .github/release.yaml at main -->



**Full Changelog**: https://github.com/d2iq-labs/capi-runtime-extensions/compare/v0.3.1...v0.3.2

## 0.3.1 (2024-02-08)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Other Changes
* build: Fix release image repository by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/337


**Full Changelog**: https://github.com/d2iq-labs/capi-runtime-extensions/compare/v0.3.0...v0.3.1

## 0.3.0 (2024-02-07)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: starts additional sec groups by @faiq in https://github.com/d2iq-labs/capi-runtime-extensions/pull/252
* feat: add control-plane load balancer scheme patch by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/228
* feat: Pull in CAAPH APIs by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/282
* feat: Use latest dynamic credential provider and v1 kubelet API by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/293
* feat: Add ClusterResourceSet strategy for CNI installation by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/288
* feat: Use CAAPH to deploy Calico on workload clusters by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/283
* feat: containerd configuration for mirror registry by @supershal in https://github.com/d2iq-labs/capi-runtime-extensions/pull/292
* feat: introduce a Go module for /api by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/331
### Fixes 🔧
* fix: Stable EBS CSI manifests by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/270
* fix: Ensure registry credentials are namespace local to Cluster by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/332
### Other Changes
* build: Upgrade devbox tools by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/271
* ci: Update release please configuration for v4 action by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/274
* build: Add release conventional commut type for release PRs by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/276
* docs: Add intro page to user docs by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/280
* build: Use ko for building OCI image by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/281
* build: Add files for clusterctl compatibility by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/284
* build: local development in macOS(and Linux) arm64/amd64 using local colima instance by @supershal in https://github.com/d2iq-labs/capi-runtime-extensions/pull/285
* build: Lint for missed errors in tests too by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/287
* build: Remove unused upx makefile stuff by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/291
* docs: Fix indentation of AWS secret example by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/294
* build: Add k8s 1.28 KinD for testing by default by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/295
* build: Add devbox update scheduled job by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/310
* build(main): Latest devbox update (2024-01-22) by @github-actions in https://github.com/d2iq-labs/capi-runtime-extensions/pull/315
* ci: Group k8s mod updates for dependabot by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/316
* build(main): Latest devbox update (2024-01-24) by @d2iq-labs-actions-pr-bot in https://github.com/d2iq-labs/capi-runtime-extensions/pull/320
* build(main): Latest devbox update (2024-02-05) by @d2iq-labs-actions-pr-bot in https://github.com/d2iq-labs/capi-runtime-extensions/pull/326
* docs: fix cluster name in README by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/330
* ci: Consistent bash defaults in workflows by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/336
* ci: Tag api module on release by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/335

## New Contributors
* @d2iq-labs-actions-pr-bot made their first contribution in https://github.com/d2iq-labs/capi-runtime-extensions/pull/320

**Full Changelog**: https://github.com/d2iq-labs/capi-runtime-extensions/compare/v0.2.0...v0.3.0

## 0.2.0 (2023-10-19)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: AWS cluster config patch by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/172
* feat: Combine generic variables with provider specific variables by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/173
* feat: Use external patch for Docker provider custom image by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/188
* feat: vendor infrastructure provider APIs by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/190
* feat: Introduce scheme and decoder helpers by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/191
* feat: add imageRegistryCredentials handler by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/174
* feat: Deploy default clusterclasses via helm by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/198
* feat: Add Calico CNI AWS ingress rules by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/206
* feat: CAPA v2.2.4 by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/211
* feat: Add worker configs var and handler by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/208
* feat: adds aws ebs config by @faiq in https://github.com/d2iq-labs/capi-runtime-extensions/pull/192
* feat: add AWS IAM instance profile patch by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/216
* feat: Calico 3.26.3 by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/218
* feat: add AWS instance type patch by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/217
* feat: variables and patches for AWS AMI spec by @supershal in https://github.com/d2iq-labs/capi-runtime-extensions/pull/225
* feat: add VPC ID and Subnet IDs patch by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/220
* feat: deploy AWS CPI by @faiq in https://github.com/d2iq-labs/capi-runtime-extensions/pull/229
### Fixes 🔧
* fix: bring back missing docker handlers by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/187
* fix: typo in docker cluster config api by @supershal in https://github.com/d2iq-labs/capi-runtime-extensions/pull/205
* fix: move provider fields under aws and docker by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/204
* fix: Correctly set external cloud provider for AWS by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/210
* fix: Adds AWS Calico installation configmap by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/212
* fix: Ensure CNI ingress rules are added to AWSCluster by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/213
* fix: Reduce log verbosity for http proxy variable not found by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/214
* fix: Don't set AWS region as required by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/219
### Other Changes
* build: Add example files to release artifacts by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/169
* build: Add AWS clusterclass example by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/162
* refactor: Move generic handlers into generic directory by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/171
* ci: Simplify shell configuration by setting defaults by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/184
* build: Disable fortify hardener to enable local debugging by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/186
* docs: Add more details about single var by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/185
* refactor: Move meta handlers to provider packages by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/193
* refactor: Use consistent decoder in mutators by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/196
* build: Suppress devbox envrc update notification by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/197
* build: Consistent behaviour in addons update scripts by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/207
* build: Allow past year in license header by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/209
* build: Increase golangci-lint timeout for slower GHA runners by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/222
* refactor: Always use unstructured in patch generators by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/221
* build: Update tools by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/223
* refactor: Remove usage of non-meta handlers by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/226

## New Contributors
* @supershal made their first contribution in https://github.com/d2iq-labs/capi-runtime-extensions/pull/205

**Full Changelog**: https://github.com/d2iq-labs/capi-runtime-extensions/compare/v0.1.2...v0.2.0

## 0.1.2 (2023-09-20)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Other Changes
* build: Use correct ghcr.io registry for multiplatform image manifest by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/167


**Full Changelog**: https://github.com/d2iq-labs/capi-runtime-extensions/compare/v0.1.1...v0.1.2

## 0.1.1 (2023-09-20)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Other Changes
* ci: Try to fix release workflow by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/165


**Full Changelog**: https://github.com/d2iq-labs/capi-runtime-extensions/compare/v0.1.0...v0.1.1

## 0.1.0 (2023-09-20)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: Use ghcr.io rather than Docker Hub by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/5
* feat: deploy Calico with ClusterResourceSet by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/9
* feat: Add helm chart by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/10
* feat: Add Flux addons provider by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/22
* feat: Delete CNI HelmRelease along with cluster by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/23
* feat: Add API boilerplate by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/25
* feat: Add ClusterAddonSet and ClusterAddon API types by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/26
* feat: Enable controller manager by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/27
* feat: delete Services type LoadBalancer on BeforeClusterDelete by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/29
* feat: Use interface to register handlers by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/98
* feat: Reintroduce manifest parser by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/101
* feat: Add version flag by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/106
* feat: Deploy calico CNI via CRS by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/107
* docs: Add starter docs site by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/112
* feat: add httpproxy external patch by @mhrabovcin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/115
* feat: Add audit policy patch by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/126
* feat: Add API server cert SANs patch by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/129
* feat: calculate default no_proxy values based on cluster by @mhrabovcin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/128
* feat: Update variable getter to handle nested fields by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/138
* feat: Support infra-specific httpproxy patches by @dlipovetsky in https://github.com/d2iq-labs/capi-runtime-extensions/pull/141
* feat: Add ClusterConfig variable and patch handler by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/142
* feat: new Kubernetes image registry patch by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/149
* feat: CNI provider deployment via variables instead of labels by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/152
* feat: add etcd registry and tag patch and vars by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/153
* feat: adds nfd  by @faiq in https://github.com/d2iq-labs/capi-runtime-extensions/pull/164
### Fixes 🔧
* fix: Fix panic when applying CNI CRS via hook by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/13
* fix: Calico deployment to work with CAPD template by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/16
* fix: Incorrect request/response parameters in CP initialized handler by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/105
* fix: Add missing AfterControlPlaneUpgradeLifecycleHandler interface by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/113
* fix: Update to latest audit policy by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/145
* fix: Do not require leader election for CAPI hooks server by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/150
* fix: typo in HTTP proxy docs by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/155
* fix: incorrect audit policy handler name by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/156
* refactor: how handlers are added to server by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/154
* fix: Handle multiple meta mutators cleanly by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/159
* fix: use repository more consistently by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/161
### Other Changes
* build: copy example from upstream by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/2
* build: Add make recipes for deploying local builds by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/11
* build: golang 1.20 by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/15
* build: Upgrade tools (#24 by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/24
* ci: Trigger checks on adding to merge queue by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/28
* build: Upgrade tools and distroless base image by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/58
* ci: Remove k8s restrictions on dependabot by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/61
* ci: Add k8s restrictions on dependabot for 0.27 by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/65
* build(deps): bump github.com/fluxcd/source-controller/api to 1.0.0-rc.1 by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/71
* refactor: Strip back to base for initial actual development by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/72
* ci: Add linting for helm chart by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/73
* build: Upgrade tools by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/82
* build: Use devbox instead of asdf by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/96
* test: Add service LB deleter test by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/99
* build: Add license headers to generated files by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/100
* build: Remove unused platform files now that devbox is used by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/103
* build: Fix up kubebuilder PROJECT file by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/102
* build: Fix up hugo mod tidy by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/122
* refactor: Use go 1.21 and new slices.Contains func by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/123
* refactor: Adopt simpler proxy generator funcs by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/124
* refactor: Move matchers to own package and add tests by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/125
* refactor: Extract server to own package for easier reuse by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/127
* test: Extract common variable testing funcs by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/131
* test: Introduce simpler patch test helpers by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/133
* refactor: Use controller manager to start runtime hooks server by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/134
* build: Upgrade everything and use nix flakes for go tools by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/135
* refactor: Move all helpers to common module by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/139
* docs: Add default extension config name in docs by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/143
* build: remove unused .tools-versions file by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/144
* ci: Dependabot for common module by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/146
* refactor: Use controller manager options for pprof handler by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/151
* build: add tooling to generate examples files by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/148
* build: Bump clusterctl to v1.5.1 and go to 1.21.1 by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/157
* ci: Explicitly specify bash as shell for GHA run steps by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/158
* docs: add new Calico variables by @dkoshkin in https://github.com/d2iq-labs/capi-runtime-extensions/pull/160
* build: Remove currently unused flux by @jimmidyson in https://github.com/d2iq-labs/capi-runtime-extensions/pull/163

## New Contributors
* @jimmidyson made their first contribution in https://github.com/d2iq-labs/capi-runtime-extensions/pull/2
* @dkoshkin made their first contribution in https://github.com/d2iq-labs/capi-runtime-extensions/pull/9
* @dependabot made their first contribution in https://github.com/d2iq-labs/capi-runtime-extensions/pull/20
* @mhrabovcin made their first contribution in https://github.com/d2iq-labs/capi-runtime-extensions/pull/115
* @dlipovetsky made their first contribution in https://github.com/d2iq-labs/capi-runtime-extensions/pull/141
* @faiq made their first contribution in https://github.com/d2iq-labs/capi-runtime-extensions/pull/164

**Full Changelog**: https://github.com/d2iq-labs/capi-runtime-extensions/commits/v0.1.0
