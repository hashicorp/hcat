// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

//+build !windows

package hcat

import (
	"os"
	"syscall"
)

func preserveFilePermissions(path string, fileInfo os.FileInfo) error {
	sysInfo := fileInfo.Sys()
	if sysInfo != nil {
		stat, ok := sysInfo.(*syscall.Stat_t)
		if ok {
			if err := os.Chown(path, int(stat.Uid), int(stat.Gid)); err != nil {
				return err
			}
		}
	}

	return nil
}
