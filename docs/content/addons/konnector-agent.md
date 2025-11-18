+++
title = "Konnector Agent Addon"
icon = "fa-solid fa-plug"
+++

The Konnector Agent addon enables automatic registration of Kubernetes clusters with Nutanix Prism Central. This addon leverages Cluster API lifecycle hooks to deploy the [Konnector Agent](https://portal.nutanix.com/page/documents/details?targetId=Prism-Central-Guide-vpc_7_3:mul-cluster-kubernetes-clusters-manage-pc-c.html) on the new clusters.

## Overview

Konnector Agent's addon management via CAREN(Cluster API Runtime Extensions - Nutanix) provides:

- **Automatic cluster registration** with Nutanix Prism Central
- **Lifecycle management** through Cluster API hooks
- **Credential management** for secure Prism Central connectivity

## Lifecycle Hooks

The addon implements the following Cluster API lifecycle hooks:

### AfterControlPlaneInitialized

- **Purpose**: Deploys the Konnector Agent after the control plane is ready
- **Timing**: Executes when the cluster control plane is fully initialized
- **Actions**:
  - Creates credentials secret on the target cluster
  - Deploys the Konnector Agent using the specified strategy
  - Configures Prism Central connectivity

### BeforeClusterUpgrade

- **Purpose**: Ensures the agent is properly configured before cluster upgrades
- **Timing**: Executes before cluster upgrade operations
- **Actions**: Re-applies the agent configuration if needed

### BeforeClusterDelete

- **Purpose**: Gracefully removes the Konnector Agent before cluster deletion
- **Timing**: Executes before cluster deletion begins
- **Actions**:
  - Initiates graceful helm uninstall
  - Waits for cleanup completion
  - Ensures proper cleanup order

## Configuration

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: my-cluster
spec:
  topology:
    variables:
      - name: clusterConfig
        value:
          addons:
            konnectorAgent:
              strategy: HelmAddon
              credentials:
                secretRef:
                  name: cluster-name-pc-credentials-for-konnector-agent
```

## Configuration Reference

### NutanixKonnectorAgent

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `strategy` | string | No | `HelmAddon` | Deployment strategy (`HelmAddon`) |
| `credentials` | object | No | - | Prism Central credentials configuration |

### NutanixKonnectorAgentCredentials

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `secretRef.name` | string | Yes | Name of the Secret containing Prism Central credentials |

## Prerequisites

### 1. Prism Central Credentials Secret

Create a secret containing Prism Central credentials:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cluster-name-pc-credentials-for-konnector-agent
  namespace: default
type: Opaque
stringData:
  username: admin
  password: password
```

### Example Configuration

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: custom-credentials-cluster
spec:
  topology:
    variables:
      - name: clusterConfig
        value:
          addons:
            konnectorAgent:
              strategy: HelmAddon
              credentials:
                secretRef:
                  name: cluster-name-pc-credentials-for-konnector-agent
```

## Default Values

The addon uses the following default values:

- **Helm Release Name**: `konnector-agent`
- **Namespace**: `ntnx-system`
- **Agent Name**: `konnector-agent`
- **Strategy**: `HelmAddon`
- **Chart**: `konnector-agent`
- **Version**: `1.3.0`

## Troubleshooting

### Common Issues

1. **Missing Credentials Secret**
   - Ensure the secret exists in the management cluster
   - Verify the secret name matches the configuration

2. **Prism Central Connectivity**
   - Check network connectivity between the cluster and Prism Central
   - Verify the Prism Central endpoint is correct
   - Ensure credentials are valid

3. **Helm Chart Issues**
   - Check the Helm repository is accessible
   - Verify the chart version exists
   - Review HelmChartProxy status

### Monitoring

Monitor the Konnector Agent deployment:

```bash
# Check HelmChartProxy status
kubectl get hcp -A

# Check agent logs
kubectl logs hook-preinstall -n ntnx-system
```

## References

- [Konnector Agent](https://portal.nutanix.com/page/documents/details?targetId=Prism-Central-Guide-vpc_7_3:mul-cluster-kubernetes-clusters-manage-pc-c.html)
- [Cluster API Add-on Provider for Helm](https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm)
- [Cluster API Runtime Hooks](https://cluster-api.sigs.k8s.io/tasks/experimental-features/runtime-sdk/hooks.html)
