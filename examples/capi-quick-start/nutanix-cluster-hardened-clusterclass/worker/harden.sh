#!/bin/bash

# Color codes
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color


echo -e "${CYAN}Listing all clusters in all namespaces...${NC}"

kubectl get clusters -A

echo ""
echo -e "${YELLOW}Please enter the namespace of the cluster you wish to harden (press Enter to use 'default'):${NC}"
read -p "> " NAMESPACE
NAMESPACE=${NAMESPACE:-default}

echo ""
echo -e "${CYAN}Using namespace: $NAMESPACE${NC}"

echo ""
echo -e "${CYAN}Listing all the KubeadmConfigTemplates for workers in namespace $NAMESPACE...${NC}"
kubectl get kubeadmconfigtemplates.bootstrap.cluster.x-k8s.io -n $NAMESPACE | grep worker

echo ""
echo -e "${YELLOW}Please enter the NKP version from the list above (e.g., for NKP worker version nkp-nutanix-worker-v2.14.0, enter v2.14.0):${NC}"
read -p "> " VERSION

echo ""
# Validate version format
if [[ ! $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo -e "${RED}Invalid version format. Please use the format v<major>.<minor>.<patch> (e.g., v2.14.1)${NC}"
  exit 1
fi

echo -e "${CYAN}Using NKP version: $VERSION${NC}"

echo ""
# Clone the latest KubeadmConfigTemplate for workers
echo -e "${CYAN}Cloning the worker template: nkp-nutanix-worker-${VERSION} from namespace $NAMESPACE...${NC}"
kubectl get kubeadmconfigtemplates.bootstrap.cluster.x-k8s.io nkp-nutanix-worker-${VERSION} -n $NAMESPACE -o yaml > nkp-nutanix-worker-${VERSION}.yaml

echo ""
# Replace <VERSION> with the actual version in all files
echo -e "${CYAN}Replacing <VERSION> with ${VERSION} in all files...${NC}"
sed -i "s/<VERSION>/${VERSION}/g" kustomization.yaml
sed -i "s/<VERSION>/${VERSION}/g" cis-mitigations-worker-patch.yaml

# Replace <NAMESPACE> with the actual namespace in all files
echo -e "${CYAN}Replacing <NAMESPACE> with ${NAMESPACE} in all files...${NC}"
sed -i "s/<NAMESPACE>/${NAMESPACE}/g" kustomization.yaml
sed -i "s/<NAMESPACE>/${NAMESPACE}/g" cis-mitigations-worker-patch.yaml

echo ""
echo -e "${GREEN}Replacement complete!${NC}"
echo -e "${GREEN}Files have been updated with version: ${VERSION} and namespace: ${NAMESPACE}${NC}"

echo ""
echo -e "${CYAN}Applying kustomization to create hardened worker template...${NC}"
kubectl apply -n $NAMESPACE -k .

echo ""
echo -e "${CYAN}You can now patch the ClusterClass to use the Hardened KubeadmConfigTemplate for workers${NC}"
echo -e "${CYAN}Here are the available ClusterClasses in namespace $NAMESPACE:${NC}"

kubectl get clusterclasses.cluster.x-k8s.io -n $NAMESPACE

echo ""
echo -e "${YELLOW}Run the below command after replacing the <CLUSTER_CLASS> with the ClusterClass in use from the list above${NC}"
echo -e "kubectl patch clusterclass -n $NAMESPACE <CLUSTER_CLASS>  \\
  --type json \\
  -p='[{\n    \"op\":\"replace\",\n    \"path\":\"/spec/workers/machineDeployments/0/template/bootstrap/ref/name\",\n    \"value\":\"nkp-nutanix-worker-${VERSION}-hardened\"\n  }]'"
