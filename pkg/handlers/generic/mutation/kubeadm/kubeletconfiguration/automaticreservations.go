// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package kubeletconfiguration

import _ "embed"

//go:embed embedded/compute-reservations.sh
var computeReservationsScript string
