#!/bin/bash

# List all the KubeadmControlPlaneTemplates to find the latest version
echo "Listing all KubeadmControlPlaneTemplates:"
kubectl get kubeadmcontrolplanetemplates.controlplane.cluster.x-k8s.io

# Prompt for the NKP version
echo "Please enter the NKP version from the list above (e.g., for NKP version nkp-nutanix-v2.14.0, enter v2.14.0):"
read -p "> " VERSION

# Validate version format
if [[ ! $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Invalid version format. Please use the format v<major>.<minor>.<patch> (e.g., v2.14.1)"
  exit 1
fi

echo "Using NKP version: $VERSION"

# Clone the Latest KubeadmControlplaneTemplate
echo "Cloning the template: nkp-nutanix-${VERSION}"
kubectl get kubeadmcontrolplanetemplates.controlplane.cluster.x-k8s.io nkp-nutanix-${VERSION} -o yaml > nkp-nutanix-${VERSION}.yaml

# Replace <VERSION> with the actual version in all files
echo "Replacing <VERSION> with ${VERSION} in all files..."
sed -i "s/<VERSION>/${VERSION}/g" kustomization.yaml
sed -i "s/<VERSION>/${VERSION}/g" cis-mitigations-cp-patch.yaml

echo "Replacement complete!"
echo "Files have been updated with version: ${VERSION}"

echo "Applying kustomization to create hardened control plane template..."
kubectl apply -k .

echo "You can now patch the ClusterClass to use the Hardened KubeadmControlPlaneTemplates"
echo "Here are the available ClusterClass"

kubectl get clusterclasses.cluster.x-k8s.io

echo "Run the below command after replacing the <CLUSTER_CLASS> with the ClusterClass in use from the list above"
echo "kubectl patch clusterclass <CLUSTER_CLASS> \\
  --type merge -p='{\"spec\":{\"controlPlane\":{\"ref\":{\"name\":\"nkp-nutanix-${VERSION}-hardened\"}}}}'"