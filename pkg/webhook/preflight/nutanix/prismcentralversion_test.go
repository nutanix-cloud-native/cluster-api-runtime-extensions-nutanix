// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

func TestPrismCentralVersionCheck_AllowsSupportedVersionsAndInternalBuilds(t *testing.T) {
	t.Parallel()

	supportedVersions := []string{
		"7.3",
		"7.3.1",
		"7.3.1.2",
		"pc.7.3",
		"pc.7.3.1",
		"pc.7.3.1.2",
		"7.5",
		"7.5.3",
		"7.5.3.4",
		"pc.7.5",
		"pc.7.5.3",
		"pc.7.5.3.4",
		"master",
		"pc.master",
	}

	for _, version := range supportedVersions {
		version := version
		t.Run(version, func(t *testing.T) {
			t.Parallel()

			cd := &checkDependencies{
				log: logr.Discard(),
				nclient: &clientWrapper{
					GetPrismCentralVersionFunc: func(ctx context.Context) (string, error) {
						return version, nil
					},
				},
			}

			check := newPrismCentralVersionCheck(context.Background(), cd)
			result := check.Run(context.Background())
			assert.True(t, result.Allowed)
			assert.False(t, result.InternalError)
			assert.Empty(t, result.Causes)
		})
	}
}

func TestPrismCentralVersionCheck_FailsUnsupportedVersions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
	}{
		{
			name:    "Older 7.x release",
			version: "pc.7.2.0.0",
		},
		{
			name:    "Legacy 2024 release",
			version: "pc.2024.2.0.1",
		},
		{
			name:    "Legacy 2025 release",
			version: "pc.2025.1.0.0",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cd := &checkDependencies{
				log: logr.Discard(),
				nclient: &clientWrapper{
					GetPrismCentralVersionFunc: func(ctx context.Context) (string, error) {
						return tt.version, nil
					},
				},
			}

			check := newPrismCentralVersionCheck(context.Background(), cd)
			result := check.Run(context.Background())
			assert.False(t, result.Allowed)
			assert.False(t, result.InternalError)
			assert.Len(t, result.Causes, 1)
			expectedMessage := fmt.Sprintf(
				"Prism Central version %q is older than the minimum supported version %s. Upgrade Prism Central to %s or later, wait for the upgrade to finish, then retry.",
				tt.version,
				minSupportedPrismCentralVersion,
				minSupportedPrismCentralVersion,
			)
			assert.Equal(t, expectedMessage, result.Causes[0].Message)
		})
	}
}

func TestPrismCentralVersionCheck_ErrorScenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		getFunc          func(ctx context.Context) (string, error)
		expectedAllowed  bool
		expectedInternal bool
		expectedMessage  func() string
	}{
		{
			name: "API error when fetching version",
			getFunc: func(ctx context.Context) (string, error) {
				return "", assert.AnError
			},
			expectedAllowed:  false,
			expectedInternal: true,
			expectedMessage: func() string {
				return fmt.Sprintf(
					"Failed to get Prism Central version: %s. This is usually a temporary error. Please retry.",
					assert.AnError,
				)
			},
		},
		{
			name: "Empty version returned",
			getFunc: func(ctx context.Context) (string, error) {
				return "", nil
			},
			expectedAllowed:  false,
			expectedInternal: false,
			expectedMessage: func() string {
				return fmt.Sprintf(
					"Prism Central reported version %q, which is not a valid version. Upgrade Prism Central to %s or later, wait for the upgrade to finish, then retry.",
					"",
					minSupportedPrismCentralVersion,
				)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cd := &checkDependencies{
				log: logr.Discard(),
				nclient: &clientWrapper{
					GetPrismCentralVersionFunc: tt.getFunc,
				},
			}

			check := newPrismCentralVersionCheck(context.Background(), cd)
			result := check.Run(context.Background())
			assert.Equal(t, tt.expectedAllowed, result.Allowed)
			assert.Equal(t, tt.expectedInternal, result.InternalError)
			assert.Len(t, result.Causes, 1)
			assert.Equal(t, tt.expectedMessage(), result.Causes[0].Message)
		})
	}
}

func TestPrismCentralVersionCheck_SkipsWithoutClient(t *testing.T) {
	t.Parallel()

	cd := &checkDependencies{
		log: logr.Discard(),
	}

	check := newPrismCentralVersionCheck(context.Background(), cd)
	result := check.Run(context.Background())
	assert.True(t, result.Allowed)
	assert.False(t, result.InternalError)
	_ = cd
}
