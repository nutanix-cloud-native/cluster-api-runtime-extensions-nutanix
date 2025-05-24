# Hardened Kubernetes Worker Node Template

## Required Component: kubelet-csr-approver

To meet the following CIS security benchmarks:

- 1.2.5 Ensure that the --kubelet-certificate-authority argument is set as appropriate
- 1.3.6 Ensure that the RotateKubeletServerCertificate argument is set to true
- Enable Kubelet Server Cert Rotation

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

**Note**: If you choose not to install the kubelet-csr-approver, you must omit the flags related to the CIS benchmarks mentioned above from your configuration from the `cis-mitigations-worker-patch.yaml` file.

## Directory Structure

This directory contains the following files:

- `harden.sh` - Automated script to simplify the hardening process (recommended method)
- `cis-mitigations-worker-patch.yaml` - Patch file containing CIS hardening configurations
- `kustomization.yaml` - Kustomization file that applies the patch and renames the template
- `nkp-nutanix-worker-<VERSION>.yaml` - The original KubeadmConfigTemplate (generated during the hardening process via `./harden.sh`)

The `harden.sh` script automates the following tasks:
1. Lists available KubeadmConfigTemplates for workers
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

**Note**: For a fully hardened cluster, you should also apply hardening to the control plane nodes by using the scripts in the `../control-plane` directory.

## CIS Mitigations Applied

The following CIS mitigations are applied to worker nodes:

### Kubelet Configuration

- **4.1.1**: Set kubelet service file permissions to 600 or more restrictive
- **4.2.4**: Disable read-only port (`read-only-port: 0`)
- **4.2.5**: Set streaming connection idle timeout (`streaming-connection-idle-timeout: 5m`)
- **4.2.6**: Enable make-iptables-util-chains (`make-iptables-util-chains: true`)
- **4.2.8**: Set appropriate event QPS (`event-qps: 5`)
- **4.2.12**: Enforce strong TLS cryptographic ciphers with an updated suite of recommended ciphers
- **4.2.13**: Set pod-max-pids limit to 4096

### File Permissions

- **4.1.1**: Set appropriate file permissions (0600) for sensitive files including:
  - kubelet.service
  - kubelet config.yaml
  - 10-kubeadm.conf
