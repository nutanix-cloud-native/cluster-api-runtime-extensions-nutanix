package nutanix

import (
	"fmt"
	"sync"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

func (n *Checker) getClusterConfig() (*variables.ClusterConfigSpec, error) {
	return sync.OnceValues(func() (*variables.ClusterConfigSpec, error) {
		if n.cluster.Spec.Topology.Variables == nil {
			return nil, nil
		}

		clusterConfig, err := variables.UnmarshalClusterConfigVariable(n.cluster.Spec.Topology.Variables)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal .variables[.name=clusterConfig]: %w", err)
		}

		return clusterConfig, nil
	})()
}

func (n *Checker) getWorkerConfigForMachineDeployment(
	md clusterv1.MachineDeploymentTopology,
) (*variables.WorkerNodeConfigSpec, error) {
	n.workerConfigGetterByMachineDeploymentNameMutex.Lock()
	defer n.workerConfigGetterByMachineDeploymentNameMutex.Unlock()

	fn, ok := n.workerConfigGetterByMachineDeploymentName[md.Name]
	if !ok {
		fn = sync.OnceValues(func() (*variables.WorkerNodeConfigSpec, error) {
			if md.Variables == nil {
				return nil, nil
			}

			workerConfig, err := variables.UnmarshalWorkerConfigVariable(md.Variables.Overrides)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal .variables.overrides[.name=workerConfig]: %w", err)
			}

			return workerConfig, nil
		})
	}
	return fn()
}
