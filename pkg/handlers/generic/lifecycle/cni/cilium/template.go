package cilium

import (
	"bytes"
	"fmt"
	"text/template"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// templateValues replaces Cluster.Spec.ControlPlaneEndpoint.Host and Cluster.Spec.ControlPlaneEndpoint.Port in Helm values text.
func templateValues(cluster *clusterv1.Cluster, text string) (string, error) {
	ciliumTemplate, err := template.New("").Parse(text)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	type input struct {
		Cluster *clusterv1.Cluster
	}

	templateInput := input{
		Cluster: cluster,
	}

	var b bytes.Buffer
	err = ciliumTemplate.Execute(&b, templateInput)
	if err != nil {
		return "", fmt.Errorf(
			"failed setting target Cluster name and namespace in template: %w",
			err,
		)
	}

	return b.String(), nil
}
