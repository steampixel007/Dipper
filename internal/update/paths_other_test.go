//go:build !windows

package update

import (
	"os"
	"testing"
)

func makeDirectoryAlias(t *testing.T, alias, target string) {
	t.Helper()
	if err := os.Symlink(target, alias); err != nil {
		t.Fatal(err)
	}
}
