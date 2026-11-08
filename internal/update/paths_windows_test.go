//go:build windows

package update

import (
	"os/exec"
	"testing"
)

func makeDirectoryAlias(t *testing.T, alias, target string) {
	t.Helper()
	if body, err := exec.Command("cmd.exe", "/c", "mklink", "/J", alias, target).CombinedOutput(); err != nil {
		t.Fatalf("create temporary junction: %s: %v", body, err)
	}
}
