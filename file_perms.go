//+build !windows

package hat

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
