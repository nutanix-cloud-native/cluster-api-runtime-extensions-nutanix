// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package encryption

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	apiserverv1 "k8s.io/apiserver/pkg/apis/config/v1"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func Test_encryptionConfigForSecretsAndConfigMaps(t *testing.T) {
	testcases := []struct {
		name      string
		providers *carenv1.EncryptionProviders
		wantErr   error
		want      *apiserverv1.ResourceConfiguration
	}{
		{
			name: "encryption configuration using all providers",
			providers: &carenv1.EncryptionProviders{
				AESCBC:    &carenv1.AESConfiguration{},
				Secretbox: &carenv1.SecretboxConfiguration{},
			},
			wantErr: nil,
			want: &apiserverv1.ResourceConfiguration{
				Resources: []string{"secrets", "configmaps"},
				Providers: []apiserverv1.ProviderConfiguration{
					{
						AESCBC: &apiserverv1.AESConfiguration{
							Keys: []apiserverv1.Key{
								{
									Name:   "key1",
									Secret: base64.StdEncoding.EncodeToString([]byte(testToken)),
								},
							},
						},
						Secretbox: &apiserverv1.SecretboxConfiguration{
							Keys: []apiserverv1.Key{
								{
									Name:   "key1",
									Secret: base64.StdEncoding.EncodeToString([]byte(testToken)),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "encryption configuration using single provider",
			providers: &carenv1.EncryptionProviders{
				AESCBC: &carenv1.AESConfiguration{},
			},
			wantErr: nil,
			want: &apiserverv1.ResourceConfiguration{
				Resources: []string{"secrets", "configmaps"},
				Providers: []apiserverv1.ProviderConfiguration{
					{
						AESCBC: &apiserverv1.AESConfiguration{
							Keys: []apiserverv1.Key{
								{
									Name:   "key1",
									Secret: base64.StdEncoding.EncodeToString([]byte(testToken)),
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			got, gErr := encryptionConfigForSecretsAndConfigMaps(
				tt.providers,
				testTokenGenerator)
			assert.Equal(t, tt.wantErr, gErr)
			assert.Equal(t, tt.want, got)
		})
	}
}
