// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package encryption

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	apiserverv1 "k8s.io/apiserver/pkg/apis/config/v1"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func Test_encryptionConfigForSecretsAndConfigMaps(t *testing.T) {
	testcases := []struct {
		name      string
		providers []carenv1.EncryptionProvider
		wantErr   error
		want      *apiserverv1.ResourceConfiguration
	}{
		{
			name:      "encryption configuration using aescbc and secretbox providers",
			providers: []carenv1.EncryptionProvider{carenv1.AESCBC, carenv1.SecretBox},
			wantErr:   nil,
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
			name:      "unsupported encryption provider",
			providers: []carenv1.EncryptionProvider{carenv1.EncryptionProvider("kmsv2")},
			wantErr:   errors.New("unknown encryption provider: kmsv2"),
			want:      nil,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			got, gErr := encryptionConfigForSecretsAndConfigMaps(tt.providers, testTokenGenerator)
			assert.Equal(t, tt.wantErr, gErr)
			assert.Equal(t, tt.want, got)
		})
	}
}
