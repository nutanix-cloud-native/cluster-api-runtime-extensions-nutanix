// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package common

import "path/filepath"

const (
	// ConfigDirOnRemote is the directory on the machine where we write CAREN configuration (e.g. scripts, patches
	// etc) as files.
	// These files are later applied by one or more commands that run on the machine.
	ConfigDirOnRemote = "/etc/caren"

	// ContainerdScriptsDirOnRemote is the directory where we write scripts that relate to containerd as files.
	// It is a subdirectory of the root config directory.
	ContainerdScriptsDirOnRemote = ConfigDirOnRemote + "/containerd"

	// ContainerdPatchDirOnRemote is the directory where we write containerd configuration patches as files.
	// It is a subdirectory of the containerd config directory.
	ContainerdPatchDirOnRemote = ConfigDirOnRemote + "/containerd/patches"
)

// ConfigFilePathOnRemote returns the absolute path of a file that CAREN deploys onto the machine.
func ConfigFilePathOnRemote(relativePath string) string {
	return filepath.Join(ConfigDirOnRemote, relativePath)
}

// ContainerPathOnRemote returns the absolute path of a script that relates to containerd on the machine.
func ContainerdScriptPathOnRemote(relativePath string) string {
	return filepath.Join(ContainerdScriptsDirOnRemote, relativePath)
}

// ContainerdPatchPathOnRemote returns the absolute path of a containerd configuration patch on the machine.
func ContainerdPatchPathOnRemote(relativePath string) string {
	return filepath.Join(ContainerdPatchDirOnRemote, relativePath)
}
