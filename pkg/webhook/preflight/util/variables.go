package nutanix

import (
	"fmt"
	"sync"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

// VariablesGetter provides methods to retrieve variables from a Cluster object.
// These methods are thread-safe and cache the results for efficiency.
type VariablesGetter struct {
	cluster                                        *clusterv1.Cluster
	workerConfigGetterByMachineDeploymentName      map[string]func() (*variables.WorkerNodeConfigSpec, error)
	workerConfigGetterByMachineDeploymentNameMutex sync.Mutex
}

func NewVariablesGetter(cluster *clusterv1.Cluster) *VariablesGetter {
	return &VariablesGetter{
		cluster: cluster,
		workerConfigGetterByMachineDeploymentName: make(
			map[string]func() (*variables.WorkerNodeConfigSpec, error),
		),
		workerConfigGetterByMachineDeploymentNameMutex: sync.Mutex{},
	}
}

// ClusterConfig retrieves the cluster configuration variables from the Cluster object.
// This method is thread-safe, and caches the result.
func (g *VariablesGetter) ClusterConfig() (*variables.ClusterConfigSpec, error) {
	return sync.OnceValues(func() (*variables.ClusterConfigSpec, error) {
		if g.cluster.Spec.Topology.Variables == nil {
			return nil, nil
		}

		clusterConfig, err := variables.UnmarshalClusterConfigVariable(g.cluster.Spec.Topology.Variables)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal .variables[.name=clusterConfig]: %w", err)
		}

		return clusterConfig, nil
	})()
}

// WorkerConfigForMachineDeployment retrieves the worker configuration variables for the given MachineDeployment.
// This method is thread-safe, and caches the result.
func (g *VariablesGetter) WorkerConfigForMachineDeployment(
	md clusterv1.MachineDeploymentTopology,
) (*variables.WorkerNodeConfigSpec, error) {
	g.workerConfigGetterByMachineDeploymentNameMutex.Lock()
	defer g.workerConfigGetterByMachineDeploymentNameMutex.Unlock()

	fn, ok := g.workerConfigGetterByMachineDeploymentName[md.Name]
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
