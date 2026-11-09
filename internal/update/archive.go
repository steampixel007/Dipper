package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const maxExpanded = 1 << 30
const maxBundleEntries = 8192

func extractTar(ctx context.Context, archive, directory, mode string) (string, string, error) {
	resolved, err := resolveUpdatePath(directory)
	if err != nil || !samePath(directory, resolved) {
		return "", "", errors.New("unsafe archive extraction directory")
	}
	input, err := os.Open(archive)
	if err != nil {
		return "", "", err
	}
	defer input.Close()
	compressed, err := gzip.NewReader(input)
	if err != nil {
		return "", "", err
	}
	defer compressed.Close()
	reader := tar.NewReader(&contextReader{ctx: ctx, reader: compressed})
	seen := make(map[string]bool)
	var total int64
	for {
		entry, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", err
		}
		name := strings.TrimSuffix(entry.Name, "/")
		if name == "" || name == "." || name == ".." || name != path.Clean(name) || strings.ContainsAny(name, "\\:") ||
			strings.HasPrefix(name, "/") || seen[name] || len(seen) >= maxBundleEntries {
			return "", "", errors.New("unsafe or duplicate tar entry")
		}
		if mode == ModeLinux && (name != "dipper" || entry.Typeflag != tar.TypeReg) {
			return "", "", errors.New("Linux update must contain only dipper")
		}
		if mode == ModeMacOS && name != "Dipper.app" && !strings.HasPrefix(name, "Dipper.app/") {
			return "", "", errors.New("macOS update contains files outside Dipper.app")
		}
		if entry.Typeflag != tar.TypeReg && entry.Typeflag != tar.TypeDir {
			return "", "", errors.New("update archives may contain only regular files and directories")
		}
		if entry.Typeflag == tar.TypeDir && entry.Size != 0 {
			return "", "", errors.New("directory entry contains unexpected data")
		}
		if entry.Size < 0 || entry.Size > maxExpanded-total {
			return "", "", errors.New("expanded update exceeds size limit")
		}
		seen[name] = true
		target := filepath.Join(directory, filepath.FromSlash(name))
		if entry.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(target, 0755); err != nil {
				return "", "", err
			}
			continue
		}
		total += entry.Size
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return "", "", err
		}
		permissions := os.FileMode(0644)
		if entry.Mode&0111 != 0 {
			permissions = 0755
		}
		output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, permissions)
		if err != nil {
			return "", "", err
		}
		copied, copyErr := io.Copy(output, reader)
		err = errors.Join(copyErr, output.Sync(), output.Close())
		if err != nil || copied != entry.Size {
			return "", "", fmt.Errorf("extract update: %w", errors.Join(err, errors.New("incomplete archive entry")))
		}
	}
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	executable := filepath.Join(directory, "dipper")
	bundle := ""
	if mode == ModeMacOS {
		bundle = filepath.Join(directory, "Dipper.app")
		executable = filepath.Join(bundle, "Contents", "MacOS", "Dipper")
		if err := safeRegular(filepath.Join(bundle, "Contents", "Info.plist")); err != nil {
			return "", "", fmt.Errorf("missing application metadata: %w", err)
		}
	}
	if err := safeRegular(executable); err != nil {
		return "", "", fmt.Errorf("missing update executable: %w", err)
	}
	info, err := os.Stat(executable)
	if err != nil || info.Size() == 0 {
		return "", "", errors.New("empty update executable")
	}
	return executable, bundle, nil
}

func BundleDigest(bundle string) (string, error) {
	resolved, err := resolveUpdatePath(bundle)
	if err != nil || !samePath(bundle, resolved) {
		return "", errors.New("unsafe application bundle path")
	}
	info, err := os.Lstat(bundle)
	if err != nil || !info.IsDir() {
		return "", errors.New("application bundle must be a directory")
	}
	hash := sha256.New()
	var total int64
	count := 0
	err = filepath.WalkDir(bundle, func(target string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		count++
		if count > maxBundleEntries {
			return errors.New("application bundle has too many files")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return errors.New("application bundle contains a link or special file")
		}
		relative, err := filepath.Rel(bundle, target)
		if err != nil {
			return err
		}
		if info.IsDir() {
			fmt.Fprintf(hash, "directory %q %o\n", filepath.ToSlash(relative), info.Mode().Perm())
			return nil
		}
		if info.Size() > maxExpanded-total {
			return errors.New("application bundle exceeds size limit")
		}
		total += info.Size()
		digest, err := fileHash(target)
		if err != nil {
			return err
		}
		fmt.Fprintf(hash, "file %q %d %o %s\n", filepath.ToSlash(relative), info.Size(), info.Mode().Perm(), digest)
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
