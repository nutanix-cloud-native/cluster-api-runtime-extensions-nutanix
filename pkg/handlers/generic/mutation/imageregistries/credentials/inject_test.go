// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_needImageRegistryCredentials(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		configs []providerConfig
		need    bool
		wantErr error
	}{
		{
			name: "ECR credentials",
			configs: []providerConfig{
				{URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com"},
			},
			need: true,
		},
		{
			name: "registry with static credentials",
			configs: []providerConfig{{
				URL:      "https://myregistry.com",
				Username: "myuser",
				Password: "mypassword",
			}},
			need: true,
		},
		{
			name: "ECR mirror",
			configs: []providerConfig{
				{
					URL:    "https://123456789.dkr.ecr.us-east-1.amazonaws.com",
					Mirror: true,
				},
			},
			need: true,
		},
		{
			name: "mirror with static credentials",
			configs: []providerConfig{{
				URL:      "https://myregistry.com",
				Username: "myuser",
				Password: "mypassword",
				Mirror:   true,
			}},
			need: true,
		},
		{
			name: "mirror with no credentials",
			configs: []providerConfig{{
				URL:    "https://myregistry.com",
				Mirror: true,
			}},
			need: false,
		},
		{
			name: "a registry with static credentials and a mirror with no credentials",
			configs: []providerConfig{
				{
					URL:      "https://myregistry.com",
					Username: "myuser",
					Password: "mypassword",
					Mirror:   true,
				},
				{
					URL:    "https://myregistry.com",
					Mirror: true,
				},
			},
			need: true,
		},
		{
			name: "registry with missing credentials",
			configs: []providerConfig{{
				URL: "https://myregistry.com",
			}},
			need:    false,
			wantErr: ErrCredentialsNotFound,
		},
	}

	for idx := range testCases {
		tt := testCases[idx]

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			need, err := needImageRegistryCredentials(tt.configs)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.need, need)
		})
	}
}
