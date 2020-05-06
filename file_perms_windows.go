//+build windows

package hat

import "os"

func preserveFilePermissions(path string, fileInfo os.FileInfo) error {
	return nil
}
