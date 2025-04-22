package distribution

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func Test_BundleLoaderObjects(t *testing.T) {
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-namespace",
			Annotations: map[string]string{
				v1alpha1.ClusterUUIDAnnotationKey: "test-cluster-uuid",
			},
		},
		Spec: clusterv1.ClusterSpec{
			Topology: &clusterv1.Topology{
				Version: "v1.30.100",
			},
		},
	}
	config := &v1alpha1.InClusterRegistryConfiguration{
		BundleLoader: &v1alpha1.RegistryBundleLoader{
			FromVolumes: []v1alpha1.RegistryBundleVolumeSource{
				{
					HostPath: &v1alpha1.HostPathVolumeSource{
						Path: "/data/bundle.tar",
					},
				},
			},
		},
	}
	registryAddress := "http://registry.example.com"

	input := &LoaderInput{
		Cluster:         cluster,
		Config:          config,
		RegistryAddress: registryAddress,
	}

	objs, err := BundleLoaderObjects(input)
	assert.NoError(t, err)
	assert.Len(t, objs, 2)
}
