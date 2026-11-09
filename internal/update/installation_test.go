package update

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstalledVersionMatchesRunningCopy(t *testing.T) {
	installed := t.TempDir()
	if err := os.WriteFile(filepath.Join(installed, "Uninstall.exe"), []byte("uninstaller fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	for _, scenario := range []struct {
		location, version, expected string
	}{
		{installed, "v0.1.5", "0.1.5"},
		{other, "v0.1.5", ""},
		{installed, "dev", ""},
	} {
		written := ""
		err := refreshInstallationVersion(filepath.Join(installed, "Dipper.exe"), scenario.version, scenario.location, func(current string) error {
			written = current
			return nil
		})
		if err != nil || written != scenario.expected {
			t.Fatalf("version=%s location=%s wrote=%s expected=%s err=%v", scenario.version, scenario.location, written, scenario.expected, err)
		}
	}
}
