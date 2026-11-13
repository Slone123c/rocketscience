//go:build !windows

package systemproxy

import "os"

func syncDir(dir string) error {
	directory, err := os.Open(dir)
	if err != nil {
		return nil
	}
	defer directory.Close()
	return directory.Sync()
}
