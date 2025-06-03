package nutanix

import (
	"context"
	"testing"

	vmmv4 "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/content"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	prismv3 "github.com/nutanix-cloud-native/prism-go-client/v3"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func TestInitCredentialsCheck_Success(t *testing.T) {
	nc := validNutanixChecker()
	check := nc.initCredentialsCheck(context.Background())
	result := check(context.Background())
	assert.True(t, result.Allowed)
	assert.False(t, result.Error)
	assert.Empty(t, result.Causes)
}

func TestInitCredentialsCheck_NoNutanixConfig(t *testing.T) {
	nc := validNutanixChecker()
	nc.nutanixClusterConfigSpec = nil
	nc.nutanixWorkerNodeConfigSpecByMachineDeploymentName = map[string]*carenv1.NutanixWorkerNodeConfigSpec{}
	check := nc.initCredentialsCheck(context.Background())
	result := check(context.Background())
	assert.True(t, result.Allowed)
	assert.False(t, result.Error)
	assert.Empty(t, result.Causes)
}

func TestInitCredentialsCheck_MissingNutanixField(t *testing.T) {
	nc := validNutanixChecker()
	nc.nutanixClusterConfigSpec.Nutanix = nil
	check := nc.initCredentialsCheck(context.Background())
	result := check(context.Background())
	assert.False(t, result.Allowed)
	assert.True(t, result.Error)
	assert.NotEmpty(t, result.Causes)
	assert.Contains(t, result.Causes[0].Message, "Nutanix cluster configuration is not defined")
}

func TestInitCredentialsCheck_InvalidURL(t *testing.T) {
	nc := validNutanixChecker()
	nc.nutanixClusterConfigSpec.Nutanix.PrismCentralEndpoint.URL = "not-a-url"
	check := nc.initCredentialsCheck(context.Background())
	result := check(context.Background())
	assert.False(t, result.Allowed)
	assert.True(t, result.Error)
	assert.Contains(t, result.Causes[0].Message, "failed to parse Prism Central endpoint URL")
}

func TestInitCredentialsCheck_SecretNotFound(t *testing.T) {
	nc := validNutanixChecker()
	nc.kclient = fake.NewClientBuilder().Build() // no secret
	check := nc.initCredentialsCheck(context.Background())
	result := check(context.Background())
	assert.False(t, result.Allowed)
	assert.True(t, result.Error)
	assert.Contains(t, result.Causes[0].Message, "failed to get Prism Central credentials Secret")
}

func TestInitCredentialsCheck_SecretEmpty(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ntnx-creds",
			Namespace: "default",
		},
		Data: map[string][]byte{},
	}
	kclient := fake.NewClientBuilder().WithObjects(secret).Build()
	nc := validNutanixChecker()
	nc.kclient = kclient
	check := nc.initCredentialsCheck(context.Background())
	result := check(context.Background())
	assert.False(t, result.Allowed)
	assert.True(t, result.Error)
	assert.Contains(t, result.Causes[0].Message, "credentials Secret 'ntnx-creds' is empty")
}

func TestInitCredentialsCheck_SecretMissingKey(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ntnx-creds",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"not-credentials": []byte("foo"),
		},
	}
	kclient := fake.NewClientBuilder().WithObjects(secret).Build()
	nc := validNutanixChecker()
	nc.kclient = kclient
	check := nc.initCredentialsCheck(context.Background())
	result := check(context.Background())
	assert.False(t, result.Allowed)
	assert.True(t, result.Error)
	assert.Contains(t, result.Causes[0].Message, "does not contain key 'credentials'")
}

func TestInitCredentialsCheck_InvalidCredentialsFormat(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ntnx-creds",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": []byte("not-a-valid-format"),
		},
	}
	kclient := fake.NewClientBuilder().WithObjects(secret).Build()
	nc := validNutanixChecker()
	nc.kclient = kclient
	check := nc.initCredentialsCheck(context.Background())
	result := check(context.Background())
	assert.False(t, result.Allowed)
	assert.True(t, result.Error)
	assert.Contains(t, result.Causes[0].Message, "failed to parse Prism Central credentials")
}

func validNutanixChecker() *nutanixChecker {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ntnx-creds",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": []byte(`[
				{
					"type": "basic_auth",
					"data": {
						"prismCentral": {
							"username": "testuser",
							"password": "testpassword"
						}
					}
				}
			]`),
		},
	}
	kclient := fake.NewClientBuilder().WithObjects(secret).Build()
	return &nutanixChecker{
		kclient: kclient,
		cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "default",
			},
		},

		v3clientFactory: func(_ prismgoclient.Credentials) (v3client, error) {
			return &mockv3client{}, nil
		},
		v4clientFactory: func(_ prismgoclient.Credentials) (v4client, error) {
			return &mockv4client{}, nil
		},

		nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
			Nutanix: &carenv1.NutanixSpec{
				PrismCentralEndpoint: carenv1.NutanixPrismCentralEndpointSpec{
					URL: "https://pc.example.com:9440",
					Credentials: carenv1.NutanixPrismCentralEndpointCredentials{
						SecretRef: carenv1.LocalObjectReference{
							Name: "ntnx-creds",
						},
					},
				},
			},
		},
		nutanixWorkerNodeConfigSpecByMachineDeploymentName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{},
	}
}

type mockv3client struct{}

func (m *mockv3client) GetCurrentLoggedInUser(ctx context.Context) (*prismv3.UserIntentResponse, error) {
	return &prismv3.UserIntentResponse{}, nil
}

type mockv4client struct {
	image  *vmmv4.GetImageApiResponse
	images *vmmv4.ListImagesApiResponse
}

func (m *mockv4client) GetImageById(id *string) (*vmmv4.GetImageApiResponse, error) {
	return m.image, nil
}

func (m *mockv4client) ListImages(
	_, _ *int,
	_, _, _ *string,
	_ ...map[string]interface{},
) (*vmmv4.ListImagesApiResponse, error) {
	return m.images, nil
}
