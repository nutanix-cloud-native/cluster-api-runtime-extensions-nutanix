#!/bin/bash

# List all the KubeadmConfigTemplates to find the latest version
echo "Listing all KubeadmConfigTemplates for workers:"
kubectl get kubeadmconfigtemplates.bootstrap.cluster.x-k8s.io | grep worker

# Prompt for the NKP version
echo "Please enter the NKP version from the list above (e.g., for NKP worker version nkp-nutanix-worker-v2.14.0, enter v2.14.0):"
read -p "> " VERSION

# Validate version format
if [[ ! $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Invalid version format. Please use the format v<major>.<minor>.<patch> (e.g., v2.14.1)"
  exit 1
fi

echo "Using NKP version: $VERSION"

# Clone the latest KubeadmConfigTemplate for workers
echo "Cloning the worker template: nkp-nutanix-worker-${VERSION}"
kubectl get kubeadmconfigtemplates.bootstrap.cluster.x-k8s.io nkp-nutanix-worker-${VERSION} -o yaml > nkp-nutanix-worker-${VERSION}.yaml

# Replace <VERSION> with the actual version in all files
echo "Replacing <VERSION> with ${VERSION} in all files..."
sed -i "s/<VERSION>/${VERSION}/g" kustomization.yaml
sed -i "s/<VERSION>/${VERSION}/g" cis-mitigations-worker-patch.yaml

echo "Replacement complete!"
echo "Files have been updated with version: ${VERSION}"

# Apply the kustomization to create the hardened worker template
echo "Applying kustomization to create hardened worker template..."
kubectl apply -k .

echo "You can now patch the ClusterClass to use the Hardened KubeadmConfigTemplate for workers"
echo "Here are the available ClusterClasses:"

kubectl get clusterclasses.cluster.x-k8s.io

echo "Run the below command after replacing the <CLUSTER_CLASS> with the ClusterClass in use from the list above"

echo "kubectl patch clusterclass <CLUSTER_CLASS> \\
  --type json \\
  -p='[{
    \"op\":\"replace\",
    \"path\":\"/spec/workers/machineDeployments/0/template/bootstrap/ref/name\",
    \"value\":\"nkp-nutanix-worker-${VERSION}-hardened\"
  }]'"
