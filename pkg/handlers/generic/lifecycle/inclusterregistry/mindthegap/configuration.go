package mindthegap

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/inclusterregistry/utils"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

const (
	namespace         = "kube-system"
	podName           = "in-cluster-registry"
	waitContainerName = "wait"

	bundleHostPathDirectoryName = "/registry-data"
	bundleFileName              = "bundle.tar"
)

var (
	//go:embed embedded/mindthegap.yaml.gotmpl
	mindthegapObjs         []byte
	mindthegapObjsTemplate = template.Must(
		template.New("").Parse(string(mindthegapObjs)),
	)

	//go:embed embedded/seeder.yaml.gotmpl
	seederObjs         []byte
	seederObjsTemplate = template.Must(
		template.New("").Parse(string(seederObjs)),
	)
)

type ConfigurationInput struct {
	Cluster *clusterv1.Cluster
}

func RemoteClusterObjects(input *ConfigurationInput) ([]unstructured.Unstructured, error) {
	serviceIP, err := utils.ServiceIPForCluster(input.Cluster)
	if err != nil {
		return nil, fmt.Errorf("error getting service IP for the distribution registry: %w", err)
	}
	var b bytes.Buffer
	templateInput := struct {
		PodName           string
		WaitContainerName string

		ServiceIP string

		BundleDirectoryName string
		BundleFileName      string
	}{
		PodName:             podName,
		WaitContainerName:   waitContainerName,
		ServiceIP:           serviceIP,
		BundleDirectoryName: bundleHostPathDirectoryName,
		BundleFileName:      bundleFileName,
	}

	err = mindthegapObjsTemplate.Execute(&b, templateInput)
	if err != nil {
		return nil, fmt.Errorf("error templaling mindthegap registry objects: %w", err)
	}

	return handlersutils.UnstructuredFromBytes(b.Bytes(), namespace)
}

func ManagementClusterObjects(input *ConfigurationInput) ([]unstructured.Unstructured, error) {
	var b bytes.Buffer
	templateInput := struct {
		JobName string

		ClusterName string

		DestinationNamespace     string
		DestinationPodName       string
		DestinationContainerName string

		BundleDirectoryName string
		BundleFileName      string
	}{
		JobName:                  seederJobNameForCluster(input.Cluster),
		ClusterName:              input.Cluster.Name,
		DestinationNamespace:     namespace,
		DestinationPodName:       podName,
		DestinationContainerName: waitContainerName,
		BundleDirectoryName:      bundleHostPathDirectoryName,
		BundleFileName:           bundleFileName,
	}

	err := seederObjsTemplate.Execute(&b, templateInput)
	if err != nil {
		return nil, fmt.Errorf("error templaling seeder objects: %w", err)
	}

	return handlersutils.UnstructuredFromBytes(b.Bytes(), input.Cluster.Namespace)
}

func seederJobNameForCluster(cluster *clusterv1.Cluster) string {
	return fmt.Sprintf("registry-seeder-%s", cluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey])
}
