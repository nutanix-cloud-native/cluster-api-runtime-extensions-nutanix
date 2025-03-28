package nutanix

import (
	"fmt"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	ccmCredentialsRequestName     = "%s-ccm-credentials-request"
	nutanixCCMCredentialsTemplate = `[
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

func NutanixPCCredentialsRequestName(clusterName string) string {
	return fmt.Sprintf(ccmCredentialsRequestName, clusterName)
}

func NutanixCCMCreentialsRequest(
	clusterName, clusterNamespace, secretName, secretNameSpace string,
) *carenv1.CredentialsRequest {
	return &carenv1.CredentialsRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: carenv1.GroupVersion.String(),
			Kind:       "CredentialsRequest",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: NutanixPCCredentialsRequestName(clusterName),
			// this should be same namespace as root secrets. we are creating root secret in kube-system for the POC.
			Namespace: "kube-system",
			Finalizers: []string{
				"foregroundDeletion",
			}, // ensure that it is not deleted if the root credentials are not deleted.
		},
		Spec: carenv1.CredentialsRequestSpec{
			ClusterRef: corev1.ObjectReference{
				Name:      clusterName,
				Namespace: clusterNamespace,
			},
			SecretRef: corev1.SecretReference{
				Name:      secretName,
				Namespace: secretNameSpace,
			},
			Mode:      carenv1.CredentialsModePassthorugh,
			Component: carenv1.ComponentNutanixCCM,
		},
	}
}

// NutanixPCCredentialsSecret creates a secret with the Nutanix Prism Central credentials.
func NutanixCCMCredentialsSecret(
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
			Namespace: defaultHelmReleaseNamespace,
			Labels: map[string]string{
				"cluster.x-k8s.io/provider": "nutanix",
				"cluster.x-k8s.io/cluster":  cluster.Name,
			},
		},
		StringData: map[string]string{
			"credentials": fmt.Sprintf(
				nutanixCCMCredentialsTemplate,
				username,
				password,
			),
		},
	}
}
