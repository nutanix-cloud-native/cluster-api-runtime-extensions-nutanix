# Changelog

## 0.27.0 (2025-02-12)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: Update COSI controller Addon by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/1043
* feat: Build with Go 1.24.0 by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/1047
### Fixes 🔧
* fix: Specify PriorityClass for Node Feature Discovery components by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/1041
* fix: Correctly configure non-mirror registry certificates by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/1039
* fix: Configure priorityClassName for Cilium Hubble by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/1045
* fix: don't generate empty _default containerd mirror file by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/1042
* fix: set priority class name for metallb by @supershal in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/1046
* fix: Correctly configure dynamic credential provider by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/1040


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.26.0...v0.27.0

## 0.26.0 (2025-02-05)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: adds new field for helm values input for cilium CNI by @manoj-nutanix in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/1011
* feat: implementation for user defined configmap for cilium addon in cluster creation by @manoj-nutanix in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/1033
* feat: update CAPI to v1.9.3 by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/1010
* feat: adds cluster's ownerref on cilium helm values source object by @manoj-nutanix in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/1034
### Fixes 🔧
* fix: correctly copy Helm charts in init container by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/1018
* fix: Use mindthegap v1.17.0 for the helm-repository container by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/1024
* fix: use republished COSI controller image by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/1022
### Other Changes
* test: wait for COSI controller to be ready by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/1014
* refactor: Remove api module dependency from common module by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/1019


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.25.0...v0.26.0

## 0.25.0 (2025-01-16)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: Virtual IP configuration to set different address/port by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/986
* feat: update addon versions by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/997
* feat: COSI controller Addon by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/1008
### Fixes 🔧
* fix: check HelmReleaseReadyCondition when status is up-to-date by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/989
* fix: update CoreDNS mapping file by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/998
* fix: validates PC IP is outside Load Balancer IP Range by @manoj-nutanix in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/1001
* fix: update COSI controller image to fix CVEs by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/1012
* fix(deps): Update Nutanix CCM Version by @thunderboltsid in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/1016
* fix: update AWS CCM to latest versions by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/1013
### Other Changes
* refactor: new waiter functionality in helmAddonApplier by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/988
* build: Properly ignore ntnx API client from dependabot by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/995


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.24.0...v0.25.0

## 0.24.0 (2024-12-02)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: Auto renewal of control plane certificates patch by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/924
* feat: Add CAREN version to ExtensionConfig by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/982
### Other Changes
* build: Latest devbox update by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/978
* build: Add CAREN release to cluster class artifacts by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/979
* build: Only consider GA releases of k8s for CoreDNS version map by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/981
* docs: Fix title of DNS customization page by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/985


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.23.1...v0.24.0

## 0.23.1 (2024-11-14)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Fixes 🔧
* fix: set container name to manager to ensure we wait for it by @faiq in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/975


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.23.0...v0.23.1

## 0.23.0 (2024-11-13)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: Build with go 1.23.3 and upgrade all tools by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/967
### Fixes 🔧
* fix: Update mindthegap to fix cert rotation by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/971
* fix: image registries with no credentials but with a CA by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/927
### Other Changes
* build: set helmRepository tag in list-images target by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/968
* build: rename caren-helm-reg to better match role by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/969


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.22.0...v0.23.0

## 0.22.0 (2024-11-06)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: set default CoreDNS version by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/959
### Fixes 🔧
* fix: Use correct filename for runtime extensions component YAML by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/960
### Other Changes
* docs: Update hugo and docsy by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/958
* refactor: Update helm registry initialization by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/961


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.21.0...v0.22.0

## 0.21.0 (2024-10-29)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: update Nutanix CSI to 3.1.0 by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/956
### Fixes 🔧
* fix: bundle correct arch for the mindthegap binary by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/954
* fix: use correct securityContext for Alpine based helm-repository Pod by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/955


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.20.0...v0.21.0

## 0.20.0 (2024-10-28)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: support configuring CoreDNS image by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/950
### Fixes 🔧
* fix: adds previous versions helm charts in this container by @faiq in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/949


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.19.0...v0.20.0

## 0.19.0 (2024-10-23)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: copy charts to pvc by @faiq in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/947
### Other Changes
* build: pass the Chart version when listing images by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/945


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.18.0...v0.19.0

## 0.18.0 (2024-10-15)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: Add feature-gates plumbing by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/919
* feat(api): Add kubernetes version to coredns version mapping by @thunderboltsid in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/939
### Fixes 🔧
* fix: Shorten readiness probe period to try to prevent races by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/930
* fix: Rename webhook container to manager by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/932
* fix: list correct registry.k8s.io/sig-storage/csi-snapshotter image by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/943
* fix: include kube-vip image in generated caren-images.txt by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/940
### Other Changes
* test(e2e): Add v1.30.5 test for Nutanix by @thunderboltsid in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/931
* build: Enable building binary only on macos by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/918
* build(deps): Update clusterctl binary to v1.8.3 by @thunderboltsid in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/929


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.17.0...v0.18.0

## 0.17.0 (2024-09-27)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: Support XValidations (CEL) for CC variables by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/916
* feat: Update addon versions by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/917
### Other Changes
* build: Fix up metadata missed in v0.16.0 release process by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/920
* build: Consistently format even generated files by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/923


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.16.0...v0.17.0

## 0.16.0 (2024-09-25)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: Build with go 1.23 by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/889
* feat: Enable Hubble Relay in Cilium deployment via CAAPH by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/899
* feat: Extract CAAPH values templates to files by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/896
* feat: Build with go 1.23.1 by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/913
* feat: Support node taints per nodepool and control plane by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/909
### Fixes 🔧
* fix: Remove deprecated toleration for node-role.kubernetes.io/master by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/895
* fix: Do not use digests for Cilium images by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/901
### Other Changes
* test: Bump Kubernetes versions for tests by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/893
* ci: images tool  by @faiq in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/822
* build: Include Calico images in image list by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/903
* build: Use upstream packages again from upstream by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/908
* ci(main): enable creating release-please PR from release branches by @supershal in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/912
* docs: Enable dark mode, add Nutanix color, and header links by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/915


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.15.0...v0.16.0

## 0.15.0 (2024-08-26)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: Upgrade to Calico v3.28.1 by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/878
### Fixes 🔧
* fix: Filter out control plane endpoint from Nutanix node IPs by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/881
### Other Changes
* ci: Support release branches for release-please by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/880


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.14.5...v0.15.0

## 0.14.5 (2024-08-21)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Other Changes
* revert: Revert upgrades for CAPI 1.8 by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/876


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.14.4...v0.14.5

## 0.14.4 (2024-08-21)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Fixes 🔧
* fix: Properly set additionalTrustBundle for Nutanix CCM by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/874


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.14.3...v0.14.4

## 0.14.3 (2024-08-20)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Fixes 🔧
* fix: CRS generated CA Deployment has extra quotes by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/867
* fix: Use same security context & priority class for helm-repository pod by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/871
### Other Changes
* build: Include debug symbols in release image executable by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/873
* build: Use new clusterctl-aws name for clusterawsadm by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/863


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.14.2...v0.14.3

## 0.14.2 (2024-08-15)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Fixes 🔧
* fix: Ensure ClusterAutoscaler can write status CM in workload cluster by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/864


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.14.1...v0.14.2

## 0.14.1 (2024-08-15)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Fixes 🔧
* fix: delete correct cluster-autoscaler HCP by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/859
* fix: skip UUID annotation webhook for clusters with nil topology by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/860
### Other Changes
* docs: Update release process for adding release series metadata by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/861


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.14.0...v0.14.1

## 0.14.0 (2024-08-14)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: Run hooks in parallel with aggregated responses by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/855
### Fixes 🔧
* fix: Cilium-Istio compatibility fixes by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/856
### Other Changes
* build: Latest devbox update by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/857


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.13.7...v0.14.0

## 0.13.7 (2024-08-13)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Fixes 🔧
* fix: Deterministic ordering of lifecycle hooks by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/847
* fix: reorder lifecycle handlers with serviceloadbalancer being last by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/848
* fix: update mindthegap by @faiq in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/852
* fix: Handle long cluster names by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/845


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.13.6...v0.13.7

## 0.13.6 (2024-08-05)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Fixes 🔧
* fix: run systemctl daemon-reload before Containerd restart by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/842


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.13.5...v0.13.6

## 0.13.5 (2024-08-02)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Fixes 🔧
* fix: sets uefi boot type by @faiq in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/838
### Other Changes
* ci: Change release-please-action to new org googleapis by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/832
* build: Derive GH owner/repo via gh by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/833
* build(deps): Upgrade CAPX Version to v1.5.0-beta.2 by @thunderboltsid in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/835
* build(e2e): Remove Kind DNS resolver override by @thunderboltsid in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/837
* build(deps): Upgrade CCM Version to v0.4.0 by @thunderboltsid in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/836


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.13.4...v0.13.5

## 0.13.4 (2024-07-29)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Fixes 🔧
* fix: ClusterAutoscaler addon ownership for move by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/830
### Other Changes
* build: Remove noisy build messages by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/826
* build(deps): Upgrade CAPX version to v1.5.0-beta.1 by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/827
* build: Latest devbox update by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/829


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.13.3...v0.13.4

## 0.13.3 (2024-07-26)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Fixes 🔧
* fix: add cluster domain with final dot to no proxy list by @mhrabovcin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/821
### Other Changes
* build: Ensure clean go build environment for goreleaser and ko by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/824


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.13.2...v0.13.3

## 0.13.2 (2024-07-23)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Fixes 🔧
* fix: Remove unused setting from Nutanix CSI chart by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/817
* fix: Use correct GA Nutanix CSI version by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/820
### Other Changes
* build: update version in metadata files by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/813


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.13.1...v0.13.2

## 0.13.1 (2024-07-19)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Fixes 🔧
* fix: generated API file by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/811
* fix: category race condition by updating Nutanix CSI to 3.0.0-2458 by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/814
* fix: use production repo for Nutanix CSI to 3.0.0-2458 by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/815


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.13.0...v0.13.1

## 0.13.0 (2024-07-18)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: Secure ciphers, min TLS v1.2, and disable auto TLS for etcd by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/808
* feat: Bump default k8s version for tests to v1.29.6 by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/784
### Fixes 🔧
* fix: add omitempty to addon strategy by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/795
* fix: update CCM to 0.3.4 to fix sweet32 issue by @tuxtof in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/805
* fix: Clean up MetalLB pod security standards labels by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/807
* fix: Fix ownership of ClusterAutoscaler resources by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/810
### Other Changes
* ci: Run e2e jobs only if unit-test, lint-*, and pre-commit jobs pass by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/796
* ci: Enable verbose output for e2e tests by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/797
* test: Verify ServiceLoadBalancer in e2e Docker and Nutanix tests by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/788
* refactor: Use CAPI conditions check where possible by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/789
* test(e2e): Use parallel tests for providers other than Docker by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/787

## New Contributors
* @tuxtof made their first contribution in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/805

**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.12.1...v0.13.0

## 0.12.1 (2024-07-05)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Fixes 🔧
* fix: Only create MetalLB configuration when necessary by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/791


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.12.0...v0.12.1

## 0.12.0 (2024-07-05)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: Add waiter for object  by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/777
* feat: Define ServiceLoadBalancer Configuration API by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/778
* feat: Use HelmAddon as default addon strategy by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/771
* feat: Apply MetalLB configuration to remote cluster by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/783
* feat: Update addon versions by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/785
### Fixes 🔧
* fix: Copy ClusterClasses and Templates without their owner references by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/776
* fix: Namespacesync controller should reconcile an updated namespace by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/775
* fix: use minimal image when deploying nfd chart by @faiq in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/774
### Other Changes
* build: Update release metadata.yaml by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/768
* ci: Run Nutanix provider e2e tests on self-hosted runner by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/755
* build: Fix devbox run errors due to piped commands by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/773
* ci: Fix ct check by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/779
* build: Use go 1.22.5 toolchain to fix CVE by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/780
* test(e2e): Use mesosphere fork v1.7.3-d2iq.1 for CAPI providers by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/781
* ci: Move govulncheck to nightly and push to main triggers by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/782
* ci: Disable nix cache on self-hosted runners by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/786


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.11.2...v0.12.0

## 0.11.2 (2024-07-01)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Fixes 🔧
* fix: Add strategy to Nutanix CCM addon in examples by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/765


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.11.1...v0.11.2

## 0.11.1 (2024-06-28)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Other Changes
* build: bring back v prefix in releases by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/760


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.11.0...v0.11.1

## 0.11.0 (2024-06-27)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: Configure namespace sync in helm chart by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/726
* feat: Support CRS for local-path provisioner and add CSI e2e by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/737
* feat: Support HelmAddon strategy for AWS EBS by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/732
* feat: Deploy snapshot-controller as separate addon by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/734
* feat: Update AWS CCM versions and add HelmAddon strategy by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/748
### Fixes 🔧
* fix: Namespace Sync controller should list no resources when source namespace is empty string by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/725
* fix: Temporarily hard-code supported PC version for Nutanix CSI by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/751
* fix: skip kubeadm CA file when Secret doesn't have a CA  by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/752
* fix: Correctly report failed deploy of ServiceLoadBalancer by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/759
### Other Changes
* build: Tidy up goreleaser config by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/745
* ci: Fix up image loading for lint-test-helm by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/746
* refactor: Tidy up Nutanix CSI with consistent apply strategy by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/733
* test(e2e): Set empty env vars for Nutanix e2e vars by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/749
* refactor: Use recommended "default" function syntax in helm templates by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/750
* refactor: Reusable HelmAddon strategy by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/735
* test(e2e): Various e2e tests fixes by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/754
* test(e2e): Correct default helm release names for AWS CCM and EBS CSI by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/756


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.10.0...v0.11.0

## 0.10.0 (2024-06-24)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: Upgrade to Cilium v1.15.5 by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/689
* feat: Upgrade to Calico v3.28.0 by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/688
* feat: bumps caaph to v0.2.3 by @faiq in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/691
* feat: Add local-path-provisioner CSI by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/693
* feat: cluster-api v1.7.3 by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/714
* feat: bumps caaph to 0.2.4 by @faiq in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/718
* feat: Controller that copies ClusterClasses to namespaces by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/715
* feat: adds a mindthegap container and deployment  by @faiq in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/637
* feat: implements BeforeClusterUpgrade hook by @faiq in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/682
### Fixes 🔧
* fix: use external Nutanix API types directly by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/698
* fix: Post-process clusterconfig CRDs for supported CSI providers by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/695
* fix: nutanix credentials Secrets owner refs by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/711
* fix: credential provider response secret ownership by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/709
* fix: static credentials Secret generation by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/717
* fix: set ownerReference on imageRegistry and globalMirror Secrets by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/720
* fix: Allow Nutanix CSI snapshot controller & webhook to run on CP nodes by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/723
* refactor: Use maps for CSI providers and storage classes by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/696
* fix: CredentialProviderConfig matchImages to support registries with port by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/724
* fix: Allow Node Feature Discovery garbage collector to run on control-plane nodes by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/722
* fix: RBAC role for namespace-sync controller to watch,list namespaces by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/738
* fix: image registries not handling CA certificates by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/729
* fix: adds a docker buildx step before release-snapshot by @faiq in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/741
### Other Changes
* docs: Add released version to helm and clusterctl install by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/683
* revert: Temporary lint config fix until next golangci-lint release (#629) by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/686
* refactor: Delete unused code by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/687
* refactor: Reduce log verbosity for skipped handlers by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/692
* build: update Go to 1.22.4 by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/700
* build(deps): Upgrade CAPX version to v1.4.0 by @thunderboltsid in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/707
* build: Move CSI supported provider logic to script by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/703
* build: Add testifylint linter by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/706
* build: Update all tools by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/704
* refactor: rename credential provider response secret by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/710
* refactor: Simplify code by using slices.Clone by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/712
* refactor: consistently use the same SetOwnerReference function by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/713
* refactor: kube-vip commands by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/699
* build: Fix an incorrect make variable passed to goreleaser by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/716
* build: Add 'chart-docs' make target by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/727
* build: Make CAREN mindthegap reg multiarch by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/730
* Add helm values schema plugin by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/728
* test(e2e): Use mesosphere fork with CRSBinding fix by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/736

## New Contributors
* @thunderboltsid made their first contribution in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/707

**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.9.0...v0.10.0

## 0.9.0 (2024-05-21)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: expose GenerateNoProxy func by @mhrabovcin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/594
* feat: Add the ServiceLoadbalancer Addon, with MetalLB as first provider by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/592
* feat: adds GPU mutation  by @faiq in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/591
* feat: Add GenericClusterConfig and add docs on usage with own CC by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/606
* feat: Enable unprivileged ports sysctl in containerd config by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/645
* feat: API for encryption at-rest by @supershal in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/610
* feat: Bump sigs.k8s.io/cluster-api to v1.7.2 by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/661
* feat: Pull calico images from quay.io instead of docker hub by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/676
* feat: update cluster autoscaler to v1.30.0 by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/681
### Fixes 🔧
* fix: Fix error messages returned by HelmChartGetter by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/598
* fix: use a consistent MachineDeployment class name by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/612
* fix: Do not return error if serviceLoadBalancer field is not set by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/611
* fix: use provided options for serverside apply by @supershal in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/627
* fix: Correct the CSI handler logic by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/603
* fix: Fix the internal ClusterConfig type used for provider-agnostic logic by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/607
* fix: log mutation failure errors by @supershal in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/649
* fix: Always apply containerd patches by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/644
* fix: cluster-autoscaler Helm values for workload clusters by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/658
* fix: Make Cluster the owner of image registry credential secret by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/648
* fix: Upgrade dynamic-credential-provider to v0.5.3 by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/677
### Other Changes
* build: Add v0.8 release metadata by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/595
* refactor: Clean up API constants, and explain usage by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/588
* docs: Add how to deploy CAREN by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/599
* docs: Upgrade hugo to latest by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/601
* docs: Update addons docs and tweak release doc by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/596
* build: Ensure provider metadata is up to date when releasing by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/600
* docs: Add how to create clusters by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/602
* docs: Update docsy module by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/605
* refactor: Apply kubebuilder annotations for required/optional everywhere by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/604
* docs: Cluster Autoscaler is deployed on the management cluster by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/608
* docs: Fix missing placeholder in "create nutanix cluster" doc by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/609
* refactor: Remove unused api/variables package by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/623
* refactor: move label helper functions to utils package by @supershal in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/626
* build: Use go1.22.3 toolchain to mitigate vulnerabilties by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/628
* build: Temporary lint config fix until next golangci-lint release by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/629
* build: Update license for Nutanix by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/456
* test(e2e): Consistent core/bootstrap/control-plane provider versions by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/639
* ci: free up disk space before running tests by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/643
* test: Add more context to panic in envtest helper by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/641
* refactor: Use colon to separate context from wrapped error by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/642
* refactor: Remove unused test helper function by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/647
* test: Add even more context to panic in envtest helper by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/650
* build: Make module-relative "go list -m" compatible with GOWORK by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/651
* test: Match cluster namespace to cluster name by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/652
* refactor: Write configuration under /etc/caren by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/656
* build: use a shorter namespace caren-system by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/662
* refactor: Use a Credentials struct consistently by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/663
* test: add encryptionAtRest config in capi-quick-start by @supershal in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/659
* test(e2e): Fix up secret ownership checks by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/665
* test: Remove hard-coded text focus and label for e2e tests by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/667
* ci: Use new dependabot multimodule capabilities by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/664
* refactor: aggregate types to be used by clients by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/672
* test: Add E2E_DRYRUN and E2E_VERBOSE make vars by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/666
* build: Ignore all gitlint rules for dependabot commits by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/675
* build: Update all tools by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/678
* test(e2e): Use upstream CRS helpers by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/680
* build: Correct dry-run output by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/679
* build: Use k8s v1.29.4 as default Kubernetes version by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/646

## New Contributors
* @prajnutanix made their first contribution in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/638

**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.8.1...v0.9.0

## 0.8.1 (2024-04-30)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Other Changes
* build: fix outdated devbox.lock file by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/590
* build: Ensure devbox.lock file is always kept up to date by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/589


**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.8.0...v0.8.1

## 0.8.0 (2024-04-29)

<!-- Release notes generated using configuration in .github/release.yaml at main -->

## What's Changed
### Exciting New Features 🎉
* feat: give mutators a clusterGetter function by @faiq in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/514
* feat: get default sans via cluster object in patch handler for docker by @faiq in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/519
* feat: adds nutanix SANs via patchHandler by @faiq in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/522
* feat: nutanix csi driver 3.0 by @faiq in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/531
* feat: Add additionalCategories field to Nutanix machine details patch by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/525
* feat: support setting Nutanix project on machines by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/535
* feat: Upgrade to CAPI v1.7.0 by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/555
* feat: CAPI v1.7.1 by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/560
* feat: Preserve user-managed fields when applying resources by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/556
* feat: Preserve user-managed fields when creating namespace by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/557
* feat: Added e2e test for capx cluster by @deepakm-ntnx in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/523
* feat: add kube-vip static Pod in a Nutanix handler by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/558
* feat: AWS CCM for Kubernetes v1.29 by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/564
### Fixes 🔧
* fix: updated the capx version used by @deepakm-ntnx in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/513
* fix: add omitempty to CCM Credentials struct by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/524
* fix: Add specific descriptions to Nutanix machine details fields by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/532
* refactor: setting ownership references to Nutanix CSI Helm Chart Proxies by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/565
### Other Changes
* build: Specify go1.22.2 as toolchain to fix govulncheck issues by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/517
* build: Add metadata for latest v0.7.0 release by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/515
* refactor: Consistently import CAPI v1beta1 package as clusterv1 alias by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/518
* build: Fix image tags in release manifests by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/516
* test(e2e): Use same versions of providers from module dependencies by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/521
* build: update aws credentials on kind bootstrap cluster by @supershal in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/507
* refactor: standardize the code for getting Helm values by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/500
* build: Use latest k8s for dev and test management cluster by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/526
* docs: Add how to release doc by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/530
* build: adds a .envrc.local file for local development for dotenv by @faiq in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/538
* refactor: create storage classes directly instead of using CRS by @faiq in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/539
* refactor: Move API to caren.nutanix.com group by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/534
* build: Add Kubernetes v1.30.0 option for bootstrap and Docker provider by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/541
* build: create .envrc.e2e file from caren e2e config by @supershal in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/540
* build: Only allow patch updates to k8s libs by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/551
* build: Generate CRD YAML by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/536
* build: Minor golangci-lint config updates for recent versions by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/552
* build: generated CRDs yamls by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/553
* refactor: Use separate types for provider cluster configs by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/537
* docs: Remove additionalCategories from required fields by @dlipovetsky in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/543
* build: Upgrade tooling, notably go to v1.22.2 by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/561
* refactor: provider an entrypoint to the infra provider meta handlers by @dkoshkin in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/554
* test(e2e): Add self-hosted e2e test by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/439
* build: Bundle k8s.io/* back in with sigs.k8s.io/* dependencies  by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/583
* build: Add envtest setup to e2e envrc by @jimmidyson in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/563

## New Contributors
* @deepakm-ntnx made their first contribution in https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pull/513

**Full Changelog**: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/compare/v0.7.0...v0.8.0

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
