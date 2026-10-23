package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVideoDefaultMigratesLegacyRootAndPreservesExistingFolders(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	want, err := DefaultDownloadRoot()
	if err != nil {
		t.Fatal(err)
	}
	for _, custom := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy default", true: "custom directory"}[custom], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.db")
			root := filepath.Join(home, "Downloads", "Dipper")
			if custom {
				root = filepath.Join(filepath.Dir(path), "chosen-directory")
			}
			c := openTest(t, path, Options{DownloadRoot: root})
			sub := create(t, c, anime, group, Rule{}, nil, nil)
			if err := c.Close(); err != nil {
				t.Fatal(err)
			}
			c, err := Open(path, Options{DisableScheduler: true})
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			settings, err := c.GetSettings()
			if err != nil {
				t.Fatal(err)
			}
			expected := want
			if custom {
				expected = root
			}
			if settings.DownloadRoot != expected {
				t.Fatalf("download root = %q, want %q", settings.DownloadRoot, expected)
			}
			if got := current(t, c, sub.ID).Directory; got != sub.Directory {
				t.Fatalf("existing anime directory changed: %q -> %q", sub.Directory, got)
			}
		})
	}
}
