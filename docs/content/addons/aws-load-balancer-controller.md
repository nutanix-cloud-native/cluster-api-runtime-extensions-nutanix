+++
title = "AWS Load Balancer Controller"
icon = "fa-solid fa-balance-scale"
+++

By leveraging CAPI cluster lifecycle hooks, this handler deploys the [AWS Load Balancer Controller] on the new cluster at the `AfterControlPlaneInitialized` phase.

The AWS Load Balancer Controller manages AWS Application Load Balancers (ALB) and Network Load Balancers (NLB) for Kubernetes services and ingresses.

Deployment of this controller is opt-in via the [provider-specific cluster configuration]({{< ref ".." >}}).

The hook uses the [Cluster API Add-on Provider for Helm] to deploy the AWS Load Balancer Controller resources.

## Prerequisites

- AWS EKS cluster
- IAM role with necessary permissions for the AWS Load Balancer Controller

## Example

To enable deployment of the AWS Load Balancer Controller on a cluster, specify the following values:

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
          addons:
            ingress:
              provider: "aws-lb-controller"
```

## IAM Permissions

The AWS Load Balancer Controller requires specific IAM permissions to manage load balancers.
See [AWS Load Balancer IAM Policies] for the full list of permissions.
The provided configuration assumes the permissions will be attached to the Nodes.

If attaching these policies to the non-default instance-profile, you may specify the name in the Cluster using the following configuration:

```yaml
        - name: workerConfig
          value:
            eks:
              ...
              iamInstanceProfile: custom.nodes.cluster-api-provider-aws.sigs.k8s.io
```

## Usage

Once deployed, the AWS Load Balancer Controller can be used to:

1. **Create Application Load Balancers (ALB)** for Kubernetes services using the `service.beta.kubernetes.io/aws-load-balancer-type: nlb` annotation
2. **Create Network Load Balancers (NLB)** for Kubernetes services using the `service.beta.kubernetes.io/aws-load-balancer-type: nlb` annotation
3. **Manage Ingress resources** with the `kubernetes.io/ingress.class: alb` annotation
4. **Configure Target Group Bindings** for advanced load balancer configurations

## Example Service

See [AWS Load Balancer NLB Example]

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-scheme: internet-facing
spec:
  type: LoadBalancer
  loadBalancerClass: service.k8s.aws/nlb
  ports:
  - port: 80
    targetPort: 8080
  selector:
    app: my-app
```

See other examples in [AWS Load Balancer Example] docs.

[AWS Load Balancer Controller]: https://kubernetes-sigs.github.io/aws-load-balancer-controller/
[Cluster API Add-on Provider for Helm]: https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm
[AWS Load Balancer IAM Policies]: https://kubernetes-sigs.github.io/aws-load-balancer-controller/latest/deploy/installation/#option-b-attach-iam-policies-to-nodes
[AWS Load Balancer NLB Example]: https://kubernetes-sigs.github.io/aws-load-balancer-controller/latest/guide/service/nlb/
[AWS Load Balancer Example]: https://kubernetes-sigs.github.io/aws-load-balancer-controller/latest/guide/ingress/annotations/
