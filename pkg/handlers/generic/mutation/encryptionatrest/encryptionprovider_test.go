// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package encryptionatrest

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	configv1 "k8s.io/apiserver/pkg/apis/config/v1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func Test_encryptionConfigForSecretsAndConfigMaps(t *testing.T) {
	testcases := []struct {
		name      string
		providers *v1alpha1.EncryptionProviders
		wantErr   error
		want      *configv1.ResourceConfiguration
	}{
		{
			name: "encryption configuration using all providers",
			providers: &v1alpha1.EncryptionProviders{
				AESCBC:    &v1alpha1.AESConfiguration{},
				Secretbox: &v1alpha1.SecretboxConfiguration{},
			},
			wantErr: nil,
			want: &configv1.ResourceConfiguration{
				Resources: []string{"secrets", "configmaps"},
				Providers: []configv1.ProviderConfiguration{
					{
						AESCBC: &configv1.AESConfiguration{
							Keys: []configv1.Key{
								{
									Name:   "key1",
									Secret: base64.StdEncoding.EncodeToString([]byte(testToken)),
								},
							},
						},
						Secretbox: &configv1.SecretboxConfiguration{
							Keys: []configv1.Key{
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
			providers: &v1alpha1.EncryptionProviders{
				AESCBC: &v1alpha1.AESConfiguration{},
			},
			wantErr: nil,
			want: &configv1.ResourceConfiguration{
				Resources: []string{"secrets", "configmaps"},
				Providers: []configv1.ProviderConfiguration{
					{
						AESCBC: &configv1.AESConfiguration{
							Keys: []configv1.Key{
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
			got, gErr := defaultEncryptionConfiguration(
				tt.providers,
				testTokenGenerator)
			assert.Equal(t, tt.wantErr, gErr)
			assert.Equal(t, tt.want, got)
		})
	}
}
