<!--
 Copyright 2023 D2iQ, Inc. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# CAPI Runtime Extensions Server

See [upstream documentation](https://cluster-api.sigs.k8s.io/tasks/experimental-features/runtime-sdk/index.html).

## Development

To deploy a local build, either initial install to update an existing deployment, run:

```shell
make dev.run-on-kind
eval $(make kind.kubeconfig)
```

Pro-tip: to redeploy without rebuilding the binaries, images, etc (useful if you have only changed the Helm chart for
example), run:

```shell
make SKIP_BUILD=true dev.run-on-kind
```

If creating an AWS cluster using the example files, you will also need to create a secret with your AWS credentials:

```shell
kubectl apply --server-side -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: "aws-quick-start-creds"
  namespace: capa-system
stringData:
 AccessKeyID: ${AWS_ACCESS_KEY_ID}
 SecretAccessKey: ${AWS_SECRET_ACCESS_KEY}
 SessionToken: ${AWS_SESSION_TOKEN}
EOF
```

If you are using an `AWS_PROFILE` to log in use the following:

```shell
kubectl apply --server-side -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: "aws-quick-start-creds"
  namespace: capa-system
stringData:
 AccessKeyID: $(aws configure get aws_access_key_id)
 SecretAccessKey: $(aws configure get aws_secret_access_key)
 SessionToken: $(aws configure get aws_session_token)
EOF
```

To create a cluster, update `clusterConfig` variable and run:

```shell
kubectl apply --server-side -f examples/capi-quick-start/docker-cluster.yaml
```

Wait until control plane is ready:

```shell
kubectl wait clusters/docker-quick-start --for=condition=ControlPlaneInitialized --timeout=5m
```

To get the kubeconfig for the new cluster, run:

```shell
clusterctl get kubeconfig docker-quick-start > docker-kubeconfig
```

If you are not on Linux, you will also need to fix the generated kubeconfig's `server`, run:

```shell
kubectl config set-cluster docker-quick-start \
  --kubeconfig docker-kubeconfig \
  --server=https://$(docker port docker-quick-start-lb 6443/tcp)
```

Wait until all nodes are ready (this indicates that CNI has been deployed successfully):

```shell
kubectl --kubeconfig docker-kubeconfig wait nodes --all --for=condition=Ready --timeout=5m
```

Show that Calico is running successfully on the workload cluster:

```shell
kubectl --kubeconfig docker-kubeconfig get daemonsets -n calico-system
```

Deploy kube-vip to provide service load-balancer:

```shell
helm repo add --force-update kube-vip https://kube-vip.github.io/helm-charts
helm repo update
kind_subnet_prefix="$(docker network inspect kind -f '{{ (index .IPAM.Config 0).Subnet }}' | \
                      grep -o '^[[:digit:]]\+\.[[:digit:]]\+\.')"
kubectl create configmap \
  --namespace kube-system kubevip \
  --from-literal "range-global=${kind_subnet_prefix}100.0-${kind_subnet_prefix}100.20" \
  --dry-run=client -oyaml |
  kubectl --kubeconfig docker-kubeconfig apply --server-side -n kube-system -f -

helm upgrade kube-vip-cloud-provider kube-vip/kube-vip-cloud-provider --version 0.2.2 \
  --install \
  --wait --wait-for-jobs \
  --namespace kube-system \
  --kubeconfig docker-kubeconfig \
  --set-string=image.tag=v0.0.6

helm upgrade kube-vip kube-vip/kube-vip --version 0.4.2 \
  --install \
  --wait --wait-for-jobs \
  --namespace kube-system \
  --kubeconfig docker-kubeconfig \
  --set-string=image.tag=v0.6.0
```

Deploy traefik as a LB service:

```shell
helm --kubeconfig docker-kubeconfig repo add traefik https://helm.traefik.io/traefik
helm repo update &>/dev/null
helm --kubeconfig docker-kubeconfig upgrade --install traefik traefik/traefik \
  --version v10.9.1 \
  --wait --wait-for-jobs \
  --set ports.web.hostPort=80 \
  --set ports.websecure.hostPort=443 \
  --set service.type=LoadBalancer
```

Watch for traefik LB service to get an external address:

```shell
watch -n 0.5 kubectl --kubeconfig docker-kubeconfig get service/traefik
```

To delete the workload cluster, run:

```shell
kubectl delete cluster docker-quick-start
```

Notice that the traefik service is deleted before the cluster is actually finally deleted.

Check the pod logs:

```shell
kubectl logs deployment/capi-runtime-extensions -f
```

To delete the dev KinD cluster, run:

```shell
make kind.delete
```
