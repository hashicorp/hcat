// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

//+build windows

package hcat

import "os"

func preserveFilePermissions(path string, fileInfo os.FileInfo) error {
	return nil
}
