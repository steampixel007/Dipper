package appdata

import "golang.org/x/sys/windows"

func platformLocalData() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_LocalAppData, windows.KF_FLAG_DONT_VERIFY)
}
