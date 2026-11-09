//go:build !windows

package update

func RefreshInstallationVersion(executable, version string) error {
	return nil
}
