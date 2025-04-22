package distribution

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	yamlMarshal "sigs.k8s.io/yaml"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

const (
	namespace         = DefaultHelmReleaseNamespace
	podName           = "registry-loader"
	waitContainerName = "wait"
)

var (
	//go:embed embedded/loader.yaml.gotmpl
	loaderObjs         []byte
	loaderObjsTemplate = template.Must(
		template.New("").Funcs(
			template.FuncMap{
				"toYAML":  toYAML,
				"nindent": nindent,
				"indent":  indent,
				"add":     add,
			},
		).Parse(string(loaderObjs)),
	)
)

// toYAML marshals any Go value to a YAML string.
func toYAML(v interface{}) (string, error) {
	b, err := yamlMarshal.Marshal(v)
	return string(b), err
}

// indent prefixes each line of s with spaces spaces.
func indent(spaces int, s string) string {
	pad := strings.Repeat(" ", spaces)
	// add pad to start of each line
	return pad + strings.ReplaceAll(s, "\n", "\n"+pad)
}

// nindent is like indent, but also prepends a newline
// so that the first line from s starts on a new indented line.
func nindent(spaces int, s string) string {
	if s == "" {
		return ""
	}
	pad := strings.Repeat(" ", spaces)
	// prefix a newline, then indent all lines
	return "\n" + pad + strings.ReplaceAll(s, "\n", "\n"+pad)
}

func add(a, b int) int {
	return a + b
}

type LoaderInput struct {
	Cluster         *clusterv1.Cluster
	Config          *v1alpha1.InClusterRegistryConfiguration
	RegistryAddress string
}

func BundleLoaderObjects(input *LoaderInput) ([]unstructured.Unstructured, error) {
	if input.Config == nil || input.Config.BundleLoader == nil {
		return nil, nil
	}

	var b bytes.Buffer
	templateInput := struct {
		JobName       string
		ConfigMapName string

		ClusterName       string
		KubernetesVersion string

		DestinationNamespace     string
		DestinationPodName       string
		DestinationContainerName string

		RegistryAddress string

		BundleFiles []v1alpha1.RegistryBundleVolumeSource
	}{
		JobName:                  copierJobNameForCluster(input.Cluster),
		ConfigMapName:            loaderConfigMapNameForCluster(input.Cluster),
		ClusterName:              input.Cluster.Name,
		KubernetesVersion:        input.Cluster.Spec.Topology.Version,
		DestinationNamespace:     namespace,
		DestinationPodName:       podName,
		DestinationContainerName: waitContainerName,
		RegistryAddress:          input.RegistryAddress,
		BundleFiles:              input.Config.BundleLoader.FromVolumes,
	}

	err := loaderObjsTemplate.Execute(&b, templateInput)
	if err != nil {
		return nil, fmt.Errorf("error templaling copier objects: %w", err)
	}

	return handlersutils.UnstructuredFromBytes(b.Bytes(), input.Cluster.Namespace)
}

func copierJobNameForCluster(cluster *clusterv1.Cluster) string {
	return fmt.Sprintf("registry-bundle-copier-%s", cluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey])
}

func loaderConfigMapNameForCluster(cluster *clusterv1.Cluster) string {
	return fmt.Sprintf("registry-loader-%s", cluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey])
}
