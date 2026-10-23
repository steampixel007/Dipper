package core

import "golang.org/x/sys/windows"

func userVideoDirectory() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_Videos, windows.KF_FLAG_DONT_VERIFY)
}
