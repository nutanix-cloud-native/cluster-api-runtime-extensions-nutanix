package distribution

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

const (
	namespace         = DefaultHelmReleaseNamespace
	podName           = "registry-loader"
	waitContainerName = "wait"

	bundleHostPathDirectoryName = "/registry-data"
	bundleFileName              = "bundle.tar"
)

var (
	//go:embed embedded/loader.yaml.gotmpl
	loaderObjs         []byte
	loaderObjsTemplate = template.Must(
		template.New("").Parse(string(loaderObjs)),
	)

	//go:embed embedded/copier.yaml.gotmpl
	copierObjs         []byte
	copierObjsTemplate = template.Must(
		template.New("").Parse(string(copierObjs)),
	)
)

type LoaderInput struct {
	Cluster *clusterv1.Cluster
}

func RegistryLoaderObjects(input *LoaderInput) ([]unstructured.Unstructured, error) {
	serviceIP, err := getServiceIP(input.Cluster.Spec.ClusterNetwork.Services.CIDRBlocks)
	if err != nil {
		return nil, fmt.Errorf("error getting service IP for the registry: %w", err)
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

	err = loaderObjsTemplate.Execute(&b, templateInput)
	if err != nil {
		return nil, fmt.Errorf("error templaling registry loader objects: %w", err)
	}

	return utils.UnstructuredFromBytes(b.Bytes(), namespace)
}

func ManagementClusterObjects(input *LoaderInput) ([]unstructured.Unstructured, error) {
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
		JobName:                  copierJobNameForCluster(input.Cluster),
		ClusterName:              input.Cluster.Name,
		DestinationNamespace:     namespace,
		DestinationPodName:       podName,
		DestinationContainerName: waitContainerName,
		BundleDirectoryName:      bundleHostPathDirectoryName,
		BundleFileName:           bundleFileName,
	}

	err := copierObjsTemplate.Execute(&b, templateInput)
	if err != nil {
		return nil, fmt.Errorf("error templaling copier objects: %w", err)
	}

	return utils.UnstructuredFromBytes(b.Bytes(), input.Cluster.Namespace)
}

func copierJobNameForCluster(cluster *clusterv1.Cluster) string {
	return fmt.Sprintf("registry-bundle-copier-%s", cluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey])
}
