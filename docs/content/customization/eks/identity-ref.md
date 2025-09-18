+++
title = "Identity Reference"
+++

The identity reference customization allows the user to specify the AWS identity to use when reconciling the EKS cluster.
This identity reference can be used to authenticate with AWS services using different identity types such as
AWSClusterControllerIdentity, AWSClusterRoleIdentity, or AWSClusterStaticIdentity.

This customization is available for EKS clusters when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

For detailed information about AWS multi-tenancy and identity management, see the
[Cluster API AWS Multi-tenancy documentation](https://cluster-api-aws.sigs.k8s.io/topics/multitenancy).

## Example

To specify the AWS identity reference for an EKS cluster, use the following configuration:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: <NAME>
spec:
  topology:
    variables:
      - name: clusterConfig
        value:
          eks:
            identityRef:
              kind: AWSClusterStaticIdentity
              name: my-aws-identity
```

## Identity Types

The following identity types are supported:

- **AWSClusterControllerIdentity**: Uses the default identity for the controller
- **AWSClusterRoleIdentity**: Assumes a role using the provided source reference
- **AWSClusterStaticIdentity**: Uses static credentials stored in a secret

## Example with Different Identity Types

### Using AWSClusterRoleIdentity

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: <NAME>
spec:
  topology:
    variables:
      - name: clusterConfig
        value:
          eks:
            identityRef:
              kind: AWSClusterRoleIdentity
              name: my-role-identity
```

### Using AWSClusterStaticIdentity

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: <NAME>
spec:
  topology:
    variables:
      - name: clusterConfig
        value:
          eks:
            identityRef:
              kind: AWSClusterStaticIdentity
              name: my-static-identity
```

Applying this configuration will result in the following value being set:

- `AWSManagedControlPlane`:

  - ```yaml
    spec:
      template:
        spec:
          identityRef:
            kind: AWSClusterStaticIdentity
            name: my-aws-identity
    ```

## Notes

- If no identity is specified, the default identity for the controller will be used
- The identity reference must exist in the cluster before creating the cluster
- For AWSClusterStaticIdentity, the referenced secret must contain the required AWS credentials
- For AWSClusterRoleIdentity, the role must be properly configured with the necessary permissions
