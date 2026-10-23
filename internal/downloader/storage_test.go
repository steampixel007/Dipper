package downloader

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
)

func TestStorageRejectsTraversalAndSymlinkEscape(t *testing.T) {
	for _, component := range []string{"..", "../escape", "..\\escape", "C:escape", "/absolute", "NUL", "CON.txt", "file.", "file ", "", "a\x00b"} {
		if err := safeComponent(component); err == nil {
			t.Fatalf("unsafe component accepted: %q", component)
		}
	}
	root := t.TempDir()
	inside, outside := filepath.Join(root, "inside"), filepath.Join(root, "outside")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(inside, "release")); err != nil {
		t.Skipf("OS does not permit symlinks: %v", err)
	}
	var failure atomic.Pointer[taskFailure]
	s := &diskStorage{folder: inside, completion: storage.NewMapPieceCompletion(), failure: &failure}
	info := &metainfo.Info{Name: "release", PieceLength: 16 << 10, Files: []metainfo.FileInfo{{Path: []string{"escaped.txt"}, Length: 1}}}
	if _, err := s.OpenTorrent(context.Background(), info, metainfo.Hash{}); err == nil {
		t.Fatal("storage followed a symlink outside the isolated folder")
	}
	if _, err := os.Stat(filepath.Join(outside, "escaped.txt")); !os.IsNotExist(err) {
		t.Fatalf("outside file was touched: %v", err)
	}
}
