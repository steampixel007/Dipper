package update

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func applicationBundle(executable string) (string, error) {
	root := filepath.Dir(filepath.Dir(filepath.Dir(executable)))
	if !strings.HasSuffix(filepath.Base(root), ".app") ||
		filepath.Dir(executable) != filepath.Join(root, "Contents", "MacOS") {
		return "", errors.New("the running executable is not inside a macOS application bundle")
	}
	return root, nil
}

func debianStageRoot() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	root := filepath.Join(cache, "Dipper", "updates")
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	if err := os.Chmod(root, 0700); err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil || !filepath.IsAbs(root) {
		return "", errors.New("invalid Debian update staging directory")
	}
	return root, nil
}

func payloadMatches(target, expected, mode string) error {
	if mode != ModeMacOS {
		return hashMatches(target, expected)
	}
	actual, err := BundleDigest(target)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) != 1 {
		return errors.New("application bundle changed after verification")
	}
	return nil
}

func removePayload(target, mode string) error {
	if mode == ModeMacOS {
		return os.RemoveAll(target)
	}
	return os.Remove(target)
}

func copyBundle(source, target string) error {
	if _, err := BundleDigest(source); err != nil {
		return err
	}
	var directories []string
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return errors.New("application bundle contains a link or special file")
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if info.IsDir() {
			if err := os.Mkdir(destination, 0700); err != nil {
				return err
			}
			directories = append(directories, relative)
			return nil
		}
		if err := copyExclusive(path, destination); err != nil {
			return err
		}
		return os.Chmod(destination, info.Mode().Perm())
	})
	if err != nil {
		return err
	}
	for index := len(directories) - 1; index >= 0; index-- {
		info, err := os.Stat(filepath.Join(source, directories[index]))
		if err != nil {
			return err
		}
		if err := os.Chmod(filepath.Join(target, directories[index]), info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

func validateUnixState(state helperState, statePath, executable string) error {
	if state.Mode != ModeMacOS && state.Mode != ModeLinux && state.Mode != ModeDebian {
		return errors.New("unsupported update mode")
	}
	if !filepath.IsAbs(state.Stage) || filepath.Clean(state.Stage) != state.Stage ||
		!strings.HasPrefix(filepath.Base(state.Stage), ".dipper-update-") ||
		statePath != filepath.Join(state.Stage, "state.json") ||
		!samePath(executable, filepath.Join(state.Stage, "helper.exe")) {
		return errors.New("invalid updater staging paths")
	}
	resolved, err := filepath.EvalSymlinks(state.Stage)
	if err != nil || resolved != state.Stage {
		return errors.New("updater stage contains a symbolic link")
	}
	for _, path := range []string{state.Target, state.WorkingDirectory, state.InstallTarget} {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || resolved != path || !filepath.IsAbs(path) {
			return errors.New("invalid canonical installation paths")
		}
	}
	info, err := os.Stat(state.WorkingDirectory)
	if err != nil || !info.IsDir() {
		return errors.New("invalid restart working directory")
	}
	if !validToken(state.TargetHash) || !validToken(state.InstallHash) ||
		!validToken(state.StagedHash) || !validToken(state.ControlToken) ||
		!validToken(state.ReadyToken) || !localAddress(state.ControlAddress) ||
		state.ParentPID <= 0 || state.ParentCreated == "" ||
		time.Since(state.Created) > 2*time.Minute || time.Until(state.Created) > time.Minute {
		return errors.New("invalid updater identity, checksum or control state")
	}
	switch state.Mode {
	case ModeMacOS:
		bundle, err := applicationBundle(state.Target)
		if err != nil || bundle != state.InstallTarget ||
			filepath.Dir(state.Stage) != filepath.Dir(bundle) ||
			state.Staged != filepath.Join(state.Stage, "new.app") {
			return errors.New("invalid application bundle update paths")
		}
	case ModeLinux:
		if state.InstallTarget != state.Target || state.InstallHash != state.TargetHash ||
			filepath.Dir(state.Stage) != filepath.Dir(state.Target) ||
			state.Staged != filepath.Join(state.Stage, "new.exe") {
			return errors.New("invalid Linux executable update paths")
		}
	case ModeDebian:
		root, err := debianStageRoot()
		if state.Target != "/opt/dipper/dipper" || state.InstallTarget != state.Target ||
			state.InstallHash != state.TargetHash ||
			state.Staged != filepath.Join(state.Stage, "package.deb") ||
			err != nil || !samePath(filepath.Dir(state.Stage), root) ||
			state.PackageHash != state.StagedHash {
			return errors.New("invalid Debian package update paths")
		}
	}
	if err := hashMatches(executable, state.TargetHash); err != nil {
		return fmt.Errorf("updater executable changed: %w", err)
	}
	if err := hashMatches(state.Target, state.TargetHash); err != nil {
		return err
	}
	if err := payloadMatches(state.InstallTarget, state.InstallHash, state.Mode); err != nil {
		return err
	}
	return payloadMatches(state.Staged, state.StagedHash, state.Mode)
}
