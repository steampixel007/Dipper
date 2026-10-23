package core

import "path/filepath"

func DefaultDownloadRoot() (string, error) {
	dir, err := userVideoDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Dipper"), nil
}
