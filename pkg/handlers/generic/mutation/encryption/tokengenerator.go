// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package encryption

import "crypto/rand"

const (
	tokenLength = 32
)

type TokenGenerator func() ([]byte, error)

func RandomTokenGenerator() ([]byte, error) {
	return createRandBytes(tokenLength)
}

// createRandBytes returns a cryptographically secure slice of random bytes with a given size.
func createRandBytes(size uint32) ([]byte, error) {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}
