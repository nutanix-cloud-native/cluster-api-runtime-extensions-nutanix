# Changelog

## 0.7.0 (2024-04-15)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: Sync up from d2iq-labs fork by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/489
* feat: set default instance profile for AWS CP and worker nodes by @supershal in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/506
### Fixes 🔧
* fix: set defaults for AWS CP and Worker instanceType by @supershal in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/504
### Other Changes
* build: Remove unused tool crane by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/459
* ci: Add govulncheck check by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/461
* ci: Remove auto-approve PR steps by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/462
* build: Tidy up examples sync script by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/458
* test: Remove redundant test case from httpproxy handler by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/463
* ci: Fix pages workflow concurrency by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/493
* refactor: Replace direct usage of CAAPH API with vendored types by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/492
* refactor: Update module paths to use nutanix-cloud-native GH org by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/494
* build: Remove unused capbk and capd hack modules by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/496
* docs: add pull request template for the repository by @supershal in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/502
* docs: Add file extension to containerd-metrics doc by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/503
* build: set dockerhub credentials for Nutanix examples by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/501


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.6.0...v0.7.0

## 0.6.0 (2024-03-19)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: Support HelmAddon strategy to deploy NFD by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/390
* feat: Upgrade AWS ESB CSI and switch to using Helm chart by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/393
* feat: CAPA 2.4.0 APIs and e2e by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/415
* feat: Single defaults namespace flag by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/426
* feat: add cluster-autoscaler CRS addon by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/423
* feat: add Cluster Autoscaler Addon with HelmAddon by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/427
* feat: NFD v0.15.2 by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/442
* feat: Include CABPK APIs by @dlipovetsky in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/445
### Fixes 🔧
* fix: Ensure addons defaults namespaces are correctly wired up by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/409
* fix: Disable hubble in Cilium deployment via CRS by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/411
* fix: Fix Cilium helm values to use kubernetes IPAM by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/413
* fix: don't use an SSH key in AWS clusters by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/425
* fix: set default priorityClassName on Deployment by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/431
* fix: set default tolerations on Deployment by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/430
* fix: Remove vendored types for core CAPI providers (CAPD, CABPK, KCP) by @dlipovetsky in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/452
### Other Changes
* test: Add initial e2e tests by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/360
* test(e2e): Add CNI e2e tests by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/383
* test(e2e): Resolve latest upstream provider releases in e2e config by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/388
* test(e2e): Add test for NFD addon by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/389
* build: Ignore controller-runtime upgrades by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/403
* test(e2e): Use ghcr.io/mesosphere/kind-node for bootstrap by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/406
* build: Update AWS CPI manifest filenames by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/410
* revert: Temporarily disable GOPROXY to workaround dodgy CAPA release (#395) by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/407
* build: Ensure release namespace is use in kustomize helm inflator by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/412
* docs: Update menu ordering and add some icons by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/414
* test(e2e): Add AWS e2e tests by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/408
* build: clusterawsadm v2.4.0 by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/424
* docs: simplify running examples in README by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/422
* ci: Add dependabot for api module by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/432
* build: Fix up third-party CAPD go.mod CAPI dependency by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/441
* build: controller-runtime v0.17.2 by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/440
* ci: Fix up release workflow by specifying workflow-dispatch version by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/451
* docs: Update docsy module by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/455
* build: Rename module to d2iq-labs/cluster-api-runtime-extensions-nutanix by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/454
* test(e2e): Update test config with new repo name by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/457
* build: Reorg example kustomizations by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/453


**Full Changelog**: https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/compare/v0.5.0...v0.6.0

## 0.5.0 (2024-02-16)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: add AWS CPI ConfigMap for v1.28 by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/376


**Full Changelog**: https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/compare/v0.4.4...v0.5.0

## 0.4.4 (2024-02-16)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Fixes 🔧
* fix: restart Containerd when setting mirror config by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/374


**Full Changelog**: https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/compare/v0.4.3...v0.4.4

## 0.4.3 (2024-02-15)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Fixes 🔧
* fix: revert back generated file changes by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/372


**Full Changelog**: https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/compare/v0.4.2...v0.4.3

## 0.4.2 (2024-02-15)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Fixes 🔧
* fix: remove word konvoy/ and use cre/ by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/365
* fix: v2/ in Containerd mirror path by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/363
* fix: set config_path in Containerd config by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/364
* fix: generate config with only globalImageRegistryMirror set by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/362
### Other Changes
* refactor: Fix formatting issue by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/368
* build: Include CAPX APis by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/367
* build: Upgrade golangci-lint to 1.56.1 by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/369
* docs: Update examples to be clusterctl templates by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/361


**Full Changelog**: https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/compare/v0.4.1...v0.4.2

## 0.4.1 (2024-02-14)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Fixes 🔧
* fix: mirror credentials support in DynamicCredentialProviderConfig by @supershal in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/359
### Other Changes
* ci: Ignore devbox update PRs in release notes by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/356
* build: Add v0.4 series to provider metadata by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/358


**Full Changelog**: https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/compare/v0.4.0...v0.4.1

## 0.4.0 (2024-02-12)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: Support multiple registry credentials if specified by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/343
* feat: Add Cilium CNI addon support by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/333
### Fixes 🔧
* fix: downgrade Calico to v3.26.4 by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/348
* fix: AMI ID patch by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/345
* fix: use correct AWS EBS CSI images by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/344
### Other Changes
* build(main): Latest devbox update (2024-02-12) by @d2iq-labs-actions-pr-bot in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/349
* build: Fix up lint config for moved external APIs by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/355
* test: add a Makefile target to update image by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/346


**Full Changelog**: https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/compare/v0.3.5...v0.4.0

## 0.3.5 (2024-02-08)

<!-- Release notes generated using configuration in .github/release.yaml at main -->



**Full Changelog**: https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/compare/v0.3.4...v0.3.5

## 0.3.4 (2024-02-08)

<!-- Release notes generated using configuration in .github/release.yaml at main -->



**Full Changelog**: https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/compare/v0.3.3...v0.3.4

## 0.3.3 (2024-02-08)

<!-- Release notes generated using configuration in .github/release.yaml at main -->



**Full Changelog**: https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/compare/v0.3.2...v0.3.3

## 0.3.2 (2024-02-08)

<!-- Release notes generated using configuration in .github/release.yaml at main -->



**Full Changelog**: https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/compare/v0.3.1...v0.3.2

## 0.3.1 (2024-02-08)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Other Changes
* build: Fix release image repository by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/337


**Full Changelog**: https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/compare/v0.3.0...v0.3.1

## 0.3.0 (2024-02-07)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: starts additional sec groups by @faiq in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/252
* feat: add control-plane load balancer scheme patch by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/228
* feat: Pull in CAAPH APIs by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/282
* feat: Use latest dynamic credential provider and v1 kubelet API by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/293
* feat: Add ClusterResourceSet strategy for CNI installation by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/288
* feat: Use CAAPH to deploy Calico on workload clusters by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/283
* feat: containerd configuration for mirror registry by @supershal in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/292
* feat: introduce a Go module for /api by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/331
### Fixes 🔧
* fix: Stable EBS CSI manifests by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/270
* fix: Ensure registry credentials are namespace local to Cluster by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/332
### Other Changes
* build: Upgrade devbox tools by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/271
* ci: Update release please configuration for v4 action by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/274
* build: Add release conventional commut type for release PRs by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/276
* docs: Add intro page to user docs by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/280
* build: Use ko for building OCI image by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/281
* build: Add files for clusterctl compatibility by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/284
* build: local development in macOS(and Linux) arm64/amd64 using local colima instance by @supershal in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/285
* build: Lint for missed errors in tests too by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/287
* build: Remove unused upx makefile stuff by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/291
* docs: Fix indentation of AWS secret example by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/294
* build: Add k8s 1.28 KinD for testing by default by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/295
* build: Add devbox update scheduled job by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/310
* build(main): Latest devbox update (2024-01-22) by @github-actions in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/315
* ci: Group k8s mod updates for dependabot by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/316
* build(main): Latest devbox update (2024-01-24) by @d2iq-labs-actions-pr-bot in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/320
* build(main): Latest devbox update (2024-02-05) by @d2iq-labs-actions-pr-bot in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/326
* docs: fix cluster name in README by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/330
* ci: Consistent bash defaults in workflows by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/336
* ci: Tag api module on release by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/335

## New Contributors
* @d2iq-labs-actions-pr-bot made their first contribution in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/320

**Full Changelog**: https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/compare/v0.2.0...v0.3.0

## 0.2.0 (2023-10-19)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: AWS cluster config patch by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/172
* feat: Combine generic variables with provider specific variables by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/173
* feat: Use external patch for Docker provider custom image by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/188
* feat: vendor infrastructure provider APIs by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/190
* feat: Introduce scheme and decoder helpers by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/191
* feat: add imageRegistryCredentials handler by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/174
* feat: Deploy default clusterclasses via helm by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/198
* feat: Add Calico CNI AWS ingress rules by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/206
* feat: CAPA v2.2.4 by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/211
* feat: Add worker configs var and handler by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/208
* feat: adds aws ebs config by @faiq in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/192
* feat: add AWS IAM instance profile patch by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/216
* feat: Calico 3.26.3 by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/218
* feat: add AWS instance type patch by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/217
* feat: variables and patches for AWS AMI spec by @supershal in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/225
* feat: add VPC ID and Subnet IDs patch by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/220
* feat: deploy AWS CPI by @faiq in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/229
### Fixes 🔧
* fix: bring back missing docker handlers by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/187
* fix: typo in docker cluster config api by @supershal in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/205
* fix: move provider fields under aws and docker by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/204
* fix: Correctly set external cloud provider for AWS by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/210
* fix: Adds AWS Calico installation configmap by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/212
* fix: Ensure CNI ingress rules are added to AWSCluster by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/213
* fix: Reduce log verbosity for http proxy variable not found by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/214
* fix: Don't set AWS region as required by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/219
### Other Changes
* build: Add example files to release artifacts by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/169
* build: Add AWS clusterclass example by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/162
* refactor: Move generic handlers into generic directory by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/171
* ci: Simplify shell configuration by setting defaults by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/184
* build: Disable fortify hardener to enable local debugging by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/186
* docs: Add more details about single var by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/185
* refactor: Move meta handlers to provider packages by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/193
* refactor: Use consistent decoder in mutators by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/196
* build: Suppress devbox envrc update notification by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/197
* build: Consistent behaviour in addons update scripts by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/207
* build: Allow past year in license header by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/209
* build: Increase golangci-lint timeout for slower GHA runners by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/222
* refactor: Always use unstructured in patch generators by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/221
* build: Update tools by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/223
* refactor: Remove usage of non-meta handlers by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/226

## New Contributors
* @supershal made their first contribution in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/205

**Full Changelog**: https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/compare/v0.1.2...v0.2.0

## 0.1.2 (2023-09-20)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Other Changes
* build: Use correct ghcr.io registry for multiplatform image manifest by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/167


**Full Changelog**: https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/compare/v0.1.1...v0.1.2

## 0.1.1 (2023-09-20)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Other Changes
* ci: Try to fix release workflow by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/165


**Full Changelog**: https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/compare/v0.1.0...v0.1.1

## 0.1.0 (2023-09-20)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: Use ghcr.io rather than Docker Hub by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/5
* feat: deploy Calico with ClusterResourceSet by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/9
* feat: Add helm chart by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/10
* feat: Add Flux addons provider by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/22
* feat: Delete CNI HelmRelease along with cluster by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/23
* feat: Add API boilerplate by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/25
* feat: Add ClusterAddonSet and ClusterAddon API types by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/26
* feat: Enable controller manager by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/27
* feat: delete Services type LoadBalancer on BeforeClusterDelete by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/29
* feat: Use interface to register handlers by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/98
* feat: Reintroduce manifest parser by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/101
* feat: Add version flag by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/106
* feat: Deploy calico CNI via CRS by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/107
* docs: Add starter docs site by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/112
* feat: add httpproxy external patch by @mhrabovcin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/115
* feat: Add audit policy patch by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/126
* feat: Add API server cert SANs patch by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/129
* feat: calculate default no_proxy values based on cluster by @mhrabovcin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/128
* feat: Update variable getter to handle nested fields by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/138
* feat: Support infra-specific httpproxy patches by @dlipovetsky in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/141
* feat: Add ClusterConfig variable and patch handler by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/142
* feat: new Kubernetes image registry patch by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/149
* feat: CNI provider deployment via variables instead of labels by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/152
* feat: add etcd registry and tag patch and vars by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/153
* feat: adds nfd  by @faiq in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/164
### Fixes 🔧
* fix: Fix panic when applying CNI CRS via hook by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/13
* fix: Calico deployment to work with CAPD template by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/16
* fix: Incorrect request/response parameters in CP initialized handler by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/105
* fix: Add missing AfterControlPlaneUpgradeLifecycleHandler interface by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/113
* fix: Update to latest audit policy by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/145
* fix: Do not require leader election for CAPI hooks server by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/150
* fix: typo in HTTP proxy docs by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/155
* fix: incorrect audit policy handler name by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/156
* refactor: how handlers are added to server by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/154
* fix: Handle multiple meta mutators cleanly by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/159
* fix: use repository more consistently by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/161
### Other Changes
* build: copy example from upstream by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/2
* build: Add make recipes for deploying local builds by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/11
* build: golang 1.20 by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/15
* build: Upgrade tools (#24 by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/24
* ci: Trigger checks on adding to merge queue by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/28
* build: Upgrade tools and distroless base image by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/58
* ci: Remove k8s restrictions on dependabot by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/61
* ci: Add k8s restrictions on dependabot for 0.27 by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/65
* build(deps): bump github.com/fluxcd/source-controller/api to 1.0.0-rc.1 by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/71
* refactor: Strip back to base for initial actual development by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/72
* ci: Add linting for helm chart by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/73
* build: Upgrade tools by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/82
* build: Use devbox instead of asdf by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/96
* test: Add service LB deleter test by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/99
* build: Add license headers to generated files by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/100
* build: Remove unused platform files now that devbox is used by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/103
* build: Fix up kubebuilder PROJECT file by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/102
* build: Fix up hugo mod tidy by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/122
* refactor: Use go 1.21 and new slices.Contains func by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/123
* refactor: Adopt simpler proxy generator funcs by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/124
* refactor: Move matchers to own package and add tests by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/125
* refactor: Extract server to own package for easier reuse by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/127
* test: Extract common variable testing funcs by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/131
* test: Introduce simpler patch test helpers by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/133
* refactor: Use controller manager to start runtime hooks server by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/134
* build: Upgrade everything and use nix flakes for go tools by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/135
* refactor: Move all helpers to common module by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/139
* docs: Add default extension config name in docs by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/143
* build: remove unused .tools-versions file by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/144
* ci: Dependabot for common module by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/146
* refactor: Use controller manager options for pprof handler by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/151
* build: add tooling to generate examples files by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/148
* build: Bump clusterctl to v1.5.1 and go to 1.21.1 by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/157
* ci: Explicitly specify bash as shell for GHA run steps by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/158
* docs: add new Calico variables by @dkoshkin in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/160
* build: Remove currently unused flux by @jimmidyson in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/163

## New Contributors
* @jimmidyson made their first contribution in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/2
* @dkoshkin made their first contribution in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/9
* @dependabot made their first contribution in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/20
* @mhrabovcin made their first contribution in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/115
* @dlipovetsky made their first contribution in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/141
* @faiq made their first contribution in https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pull/164

**Full Changelog**: https://github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/commits/v0.1.0
