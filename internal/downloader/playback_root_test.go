package downloader

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestExistingFileWithSizeRejectsSymlinkEscape pins the task-root
// confinement of file resolution: a symlinked directory pointing outside the
// task folder must never offer its contents for playback. Windows machines
// without symlink privilege fall back to a directory junction, which is the
// same reparse-point escape and needs no elevation.
func TestExistingFileWithSizeRejectsSymlinkEscape(t *testing.T) {
	folder := t.TempDir()
	outside := filepath.Join(t.TempDir(), "escape")
	if err := os.MkdirAll(outside, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "evil.exe"), []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "real.mkv"), []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(folder, "escape")
	if err := os.Symlink(outside, link); err != nil {
		if runtime.GOOS != "windows" {
			t.Skipf("symlinks unavailable on %s: %v", runtime.GOOS, err)
		}
		out, err := exec.Command("cmd", "/c", "mklink", "/J", link, outside).CombinedOutput()
		if err != nil {
			t.Skipf("symlinks and junctions unavailable: %v: %s", err, out)
		}
	}
	if path, _, ok := existingFileWithSize(folder, filepath.Join("escape", "evil.exe")); ok {
		t.Fatalf("symlink escape offered for playback: %q", path)
	}
	path, size, ok := existingFileWithSize(folder, "real.mkv")
	if !ok {
		t.Fatal("regular file inside the task root was rejected")
	}
	if path != filepath.Join(folder, "real.mkv") || size != 5 {
		t.Fatalf("regular file mismatch: %q %d", path, size)
	}
}
