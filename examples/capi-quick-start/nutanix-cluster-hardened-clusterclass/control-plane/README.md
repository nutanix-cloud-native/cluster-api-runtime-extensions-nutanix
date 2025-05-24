# Hardened Kubernetes Control Plane Template

## Required Component: kubelet-csr-approver

To meet the following CIS security benchmarks:

- 1.2.5 Ensure that the --kubelet-certificate-authority argument is set as appropriate
- 1.3.6 Ensure that the RotateKubeletServerCertificate argument is set to true

You must install the Postfinance kubelet-csr-approver:

```bash
helm repo add kubelet-csr-approver https://postfinance.github.io/kubelet-csr-approver
helm upgrade --install kubelet-csr-approver \
  kubelet-csr-approver/kubelet-csr-approver \
  -n kube-system \
  --create-namespace \
  --set maxExpirationSeconds=2592000 \
  --set leaderElection=true \
  --set bypassDnsResolution=true \
  --set rbac.create=true
```

**Note**: If you choose not to install the kubelet-csr-approver, you must omit the flags related to the CIS benchmarks mentioned above from your configuration from the `cis-mitigations-cp-patch.yaml` file.

## Directory Structure

This directory contains the following files:

- `harden.sh` - Automated script to simplify the hardening process (recommended method)
- `cis-mitigations-cp-patch.yaml` - Patch file containing CIS hardening configurations
- `kustomization.yaml` - Kustomization file that applies the patch and renames the template
- `nkp-nutanix-<VERSION>.yaml` - The original KubeadmControlPlaneTemplate (generated during the hardening process via `./harden.sh`)

The `harden.sh` script automates the following tasks:
1. Lists available KubeadmControlPlaneTemplates
2. Prompts for the NKP version
3. Exports the original template
4. Updates all version placeholders in configuration files
5. Applies the kustomization to create the hardened template
6. Provides guidance on patching the ClusterClass to use the hardened template

## Applying the Hardening

Simply run the hardening script and follow the prompts. Ensure that you have the `KUBECONFIG` environment variable set to the Management Cluster (or Self-Managed) before running it:

```bash
#export KUBECONFIG=<MANAGEMENT_CLUSTER_KUBECONFIG>
./harden.sh
```

This script will guide you through the process, automatically generate the required files, and apply the kustomization.

**Note**: For a fully hardened cluster, you should also apply hardening to the worker nodes by using the scripts in the `../worker` directory.

## CIS Mitigations Applied

The following CIS mitigations are applied to the Control Plane Nodes:

### API Server

- **1.2.15**: Disabled profiling for API server (`profiling: "false"`)
- **1.2.21**: Enabled service account lookup (`service-account-lookup: "true"`)
- **1.2.3, 1.2.9, 1.2.11, 1.2.14**: Added admission plugins:
  - AlwaysPullImages: Enforces that images are always pulled prior to starting containers
  - DenyServiceExternalIPs: Prevents services from using arbitrary external IPs
  - EventRateLimit: Mitigates event flooding attacks
  - NodeRestriction: Limits node access to specific APIs
- **1.2.9**: Configured EventRateLimit with appropriate admission control config file
- **1.2.5**: Set kubelet certificate authority (`kubelet-certificate-authority: /etc/kubernetes/pki/ca.crt`)
  - **Note**: This requires kubelet-csr-approver to be installed. If not installed, this flag should be omitted.

### Controller Manager

- **1.3.1**: Set terminated pod GC threshold to 10000 for better garbage collection
- **1.3.2**: Disabled profiling (`profiling: "false"`)
- **1.3.6**: Enabled RotateKubeletServerCertificate feature gate (`feature-gates: RotateKubeletServerCertificate=true`)
  - **Note**: This requires kubelet-csr-approver to be installed. If not installed, this flag should be omitted.

### Scheduler

- **1.4.1**: Disabled profiling (`profiling: "false"`)

### Kubelet Configuration (Both Init and Join)

- **1.2.5, 1.3.6**: Enabled kubelet server certificate rotation (`rotate-server-certificates: "true"`)
  - **Note**: This requires kubelet-csr-approver to be installed. If not installed, this flag should be omitted.

### EventRateLimit Configuration

- **1.2.9**: Created admission configuration files with detailed rate limits:
  - Server-wide: 5000 QPS with 20000 burst
  - Namespace: 500 QPS with 2000 burst (1000 cache size)
  - User: 100 QPS with 400 burst (2000 cache size)
  - SourceAndObject: 50 QPS with 100 burst (5000 cache size)

### File Permissions

- **4.1.1**: Set appropriate file permissions (0600) for sensitive files including:
  - kubelet.service
  - kubelet config.yaml
  - 10-kubeadm.conf

