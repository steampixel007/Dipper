//go:build !windows

package downloader

import (
	"fmt"
	"os"
	"syscall"
)

func isReparse(info os.FileInfo) bool {
	return info.Mode()&os.ModeSymlink != 0
}

func singleLink(file *os.File) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink != 1 {
		return fmt.Errorf("unsafe linked payload: %s", file.Name())
	}
	return nil
}
