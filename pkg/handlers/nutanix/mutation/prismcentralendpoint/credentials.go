package prismcentralendpoint

import (
	"fmt"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	pcCredentialsNameFormat      = "%s-pc-credentials"
	pcCredentialsRequestName     = "%s-pc-credentials-request"
	nutanixPCCredentialsTemplate = `[
    {
        "type": "basic_auth",
        "data": {
            "prismCentral": {
                "username": %q,
                "password": %q
            }
        }
    }
]`
)

func NutanixPCCredentialsSecretName(clusterName string) string {
	return fmt.Sprintf(pcCredentialsNameFormat, clusterName)
}

func NutanixPCCredentialsRequestName(clusterName string) string {
	return fmt.Sprintf(pcCredentialsRequestName, clusterName)
}

// NutanixPCCredentialsSecret creates a secret with the Nutanix Prism Central credentials.
func NutanixPCCredentialsSecret(
	secretName, username, password string,
	cluster *clusterv1.Cluster,
) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				"cluster.x-k8s.io/provider": "nutanix",
				"cluster.x-k8s.io/cluster":  cluster.Name,
			},
		},
		StringData: map[string]string{
			"credentials": fmt.Sprintf(
				nutanixPCCredentialsTemplate,
				username,
				password,
			),
		},
	}
}

/*
apiVersion: caren.nutanix.com/v1alpha1
kind: CredentialsRequest
metadata:
  labels:
    app.kubernetes.io/name: cluster-api-runtime-extensions-nutanix
  name: test-pc-credentials-request
spec:
  clusterSelector:
    matchLabels:
      cluster.x-k8s.io/cluster-name: shalin-upgrade-addons
      cluster.x-k8s.io/provider: nutanix
  secretRef:
    name: test-pc-credentials
    namespace: default
  type: nutanix/pc
  secretPlacement: local
*/

func NutanixPCCreentialsRequest(
	clusterName, secretName, clusterNamespace string,
) *carenv1.CredentialsRequest {
	return &carenv1.CredentialsRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: carenv1.GroupVersion.String(),
			Kind:       "CredentialsRequest",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      NutanixPCCredentialsRequestName(clusterName),
			Namespace: "default", // it should be same namespace as the CAREN. This is a placeholder.
			Finalizers: []string{
				"foregroundDeletion",
			}, // ensure that it is not deleted if the root credentials are not deleted.
		},
		Spec: carenv1.CredentialsRequestSpec{
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"cluster.x-k8s.io/provider":     "nutanix",
					"cluster.x-k8s.io/cluster-name": clusterName,
				},
			},
			SecretRef: corev1.SecretReference{
				Name:      secretName,
				Namespace: clusterNamespace,
			},
			Mode:               carenv1.CredentialsModePassthorugh,
			Component:          carenv1.ComponentNutanixCluster,
			Infrastructure:     carenv1.InfrastructureNutanix,
			RootCredentialsKey: carenv1.RootCredentialsKeyPCEndpoint,
		},
	}
}
