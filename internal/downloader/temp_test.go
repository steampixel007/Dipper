package downloader

import (
	"path/filepath"
	"testing"
)

// canonicalTempDir matches the canonical form admission persists. TempDir can
// return an alias, such as /var versus /private/var on macOS or a short name
// on Windows, while admission stores the resolved path.
func canonicalTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	canonical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return canonical
}
