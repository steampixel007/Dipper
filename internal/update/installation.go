package update

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func refreshInstallationVersion(executable, version, location string, write func(string) error) error {
	if Development(version) {
		return nil
	}
	if _, _, err := parseVersion(version); err != nil {
		return err
	}
	parent, err := resolveUpdatePath(filepath.Dir(executable))
	if err != nil {
		return err
	}
	installed, err := resolveUpdatePath(location)
	if err != nil {
		return fmt.Errorf("resolve registered installation: %w", err)
	}
	if !samePath(parent, installed) {
		return nil
	}
	uninstaller := filepath.Join(parent, "Uninstall.exe")
	if err := safeRegular(uninstaller); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return write(strings.TrimPrefix(version, "v"))
}
