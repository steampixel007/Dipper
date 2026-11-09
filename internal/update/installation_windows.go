package update

import (
	"errors"

	"golang.org/x/sys/windows/registry"
)

func RefreshInstallationVersion(executable, version string) error {
	if Development(version) {
		return nil
	}
	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Uninstall\io.github.steampixel007.dipper`,
		registry.QUERY_VALUE|registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer key.Close()
	location, _, err := key.GetStringValue("InstallLocation")
	if err != nil {
		return err
	}
	return refreshInstallationVersion(executable, version, location, func(current string) error {
		return key.SetStringValue("DisplayVersion", current)
	})
}
