package update

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemTempAliasKeepsUpdatePathsSafe(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	storage := filepath.Join(root, "storage")
	if err := os.Mkdir(storage, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "system-temp")
	makeDirectoryAlias(t, alias, storage)
	t.Cleanup(func() { os.Remove(alias) })
	for _, name := range []string{"TEMP", "TMP", "TMPDIR"} {
		t.Setenv(name, alias)
	}
	if !samePath(os.TempDir(), storage) {
		t.Fatal("the OS temporary alias does not identify the same directory")
	}
	target := filepath.Join(alias, "Dipper.exe")
	archive := filepath.Join(storage, "update.zip")
	if err := os.WriteFile(archive, fixtureZip(t, "Dipper.exe"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Extract(context.Background(), archive, target); err != nil {
		t.Fatalf("system temporary alias blocked safe extraction: %v", err)
	}
	digest, err := Digest(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyDownload(Download{Package: target, PackageSHA256: digest}); err != nil {
		t.Fatalf("system temporary alias blocked verified package: %v", err)
	}
	if !samePath(target, filepath.Join(storage, "Dipper.exe")) {
		t.Fatal("restart identity changed through the system temporary alias")
	}
	first, err := gatePath(filepath.Join(alias, "absent.exe"))
	if err != nil {
		t.Fatalf("system temporary alias blocked launch gate: %v", err)
	}
	second, err := gatePath(filepath.Join(storage, "absent.exe"))
	if err != nil || first != second {
		t.Fatalf("aliases selected different launch gates while target absent: %q %q %v", first, second, err)
	}
	escape := filepath.Join(root, "unrelated")
	if err := os.Mkdir(escape, 0700); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(storage, "redirect")
	makeDirectoryAlias(t, nested, escape)
	t.Cleanup(func() { os.Remove(nested) })
	if err := os.WriteFile(filepath.Join(escape, "Dipper.exe"), []byte("protected fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := safeRegular(filepath.Join(alias, "redirect", "Dipper.exe")); err == nil {
		t.Fatal("accepted a redirected file below the system temporary root")
	}
	if err := Extract(context.Background(), archive, filepath.Join(alias, "redirect", "Dipper.exe")); err == nil || !strings.Contains(err.Error(), "unsafe extraction directory") {
		t.Fatalf("did not reject redirected extraction directory before writing: %v", err)
	}
	if body, err := os.ReadFile(filepath.Join(escape, "Dipper.exe")); err != nil || string(body) != "protected fixture" {
		t.Fatalf("redirect rejection changed unrelated data: %q %v", body, err)
	}
	lockDirectory := launchLockDirectory(storage, os.Getuid())
	if err := os.Rename(lockDirectory, lockDirectory+"-original"); err != nil {
		t.Fatal(err)
	}
	makeDirectoryAlias(t, lockDirectory, escape)
	t.Cleanup(func() { os.Remove(lockDirectory) })
	if _, err := gatePath(target); err == nil {
		t.Fatal("accepted a redirected launch-lock directory")
	}
}

func TestUpdateRejectsDirectoryRedirects(t *testing.T) {
	root, err := resolveUpdatePath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	storage := filepath.Join(root, "storage")
	if err := os.Mkdir(storage, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "redirect")
	makeDirectoryAlias(t, alias, storage)
	t.Cleanup(func() { os.Remove(alias) })
	target := filepath.Join(storage, "Dipper.exe")
	if err := os.WriteFile(target, []byte("protected fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := safeRegular(target); err != nil {
		t.Fatal("rejected ordinary fixture:", err)
	}
	if err := safeRegular(filepath.Join(alias, "Dipper.exe")); err == nil {
		t.Fatal("accepted a redirected executable outside the trusted temporary root")
	}
	archive := filepath.Join(root, "update.zip")
	if err := os.WriteFile(archive, fixtureZip(t, "Dipper.exe"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Extract(context.Background(), archive, filepath.Join(alias, "Dipper.exe")); err == nil || !strings.Contains(err.Error(), "unsafe extraction directory") {
		t.Fatalf("did not reject redirected extraction directory before writing: %v", err)
	}
}

func TestLaunchLockDirectoryIsUserScoped(t *testing.T) {
	root := t.TempDir()
	first := launchLockDirectory(root, 1000)
	second := launchLockDirectory(root, 1001)
	if first == second {
		t.Fatalf("distinct users share a launch-lock directory: %q", first)
	}
	shared := filepath.Join(root, "Dipper-update-locks")
	if first == shared || second == shared {
		t.Fatal("launch-lock directory is shared across users")
	}
	if filepath.Dir(first) != root || filepath.Dir(second) != root {
		t.Fatalf("launch-lock directory escaped its root: %q %q", first, second)
	}
	if got := launchLockDirectory(root, -1); got != shared {
		t.Fatalf("platform without a user id lost its existing lock path: %q", got)
	}
	if got, err := gatePath(filepath.Join(root, "Dipper.exe")); err != nil || filepath.Dir(got) != launchLockDirectory(os.TempDir(), os.Getuid()) {
		t.Fatalf("launch gate does not use the user-scoped lock directory: %q %v", got, err)
	}
}
