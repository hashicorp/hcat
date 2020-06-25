//+build windows

package hcat

import "os"

func preserveFilePermissions(path string, fileInfo os.FileInfo) error {
	return nil
}
