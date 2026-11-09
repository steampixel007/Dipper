//go:build darwin || linux

package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

func preparePlatformUpdate(ctx context.Context, target string, downloaded Download, args []string) (*Installer, error) {
	mode := downloaded.Mode
	if (runtime.GOOS == "darwin" && mode != ModeMacOS) ||
		(runtime.GOOS == "linux" && mode != ModeLinux && mode != ModeDebian) {
		return nil, errors.New("update package does not match this platform")
	}
	target, err := filepath.EvalSymlinks(target)
	if err != nil || !filepath.IsAbs(target) {
		return nil, errors.New("cannot resolve installed executable")
	}
	executable, err := os.Executable()
	if err != nil || !samePath(target, executable) {
		return nil, errors.New("update target must be the running executable")
	}
	if err := safeRegular(target); err != nil {
		return nil, err
	}
	installTarget := target
	if mode == ModeMacOS {
		installTarget, err = applicationBundle(target)
		if err != nil {
			return nil, err
		}
	} else {
		owned, err := linuxPackageOwner(ctx, target)
		if err != nil {
			return nil, err
		}
		if mode == ModeLinux && owned != "" {
			return nil, fmt.Errorf("executable belongs to Debian package %s; use the authorized Debian package update", owned)
		}
		if mode == ModeDebian {
			installed, err := DebianInstallation(ctx, target)
			if err != nil || !installed || owned != "dipper" {
				return nil, errors.New("Debian updates require the installed dipper package at /opt/dipper/dipper")
			}
			if _, err := os.Stat("/usr/bin/pkexec"); err != nil {
				return nil, errors.New("Debian upgrade requires /usr/bin/pkexec and a graphical polkit authorization agent")
			}
		}
	}
	root := filepath.Dir(installTarget)
	if mode == ModeDebian {
		root, err = debianStageRoot()
		if err != nil {
			return nil, err
		}
	}
	stage, err := os.MkdirTemp(root, ".dipper-update-")
	if err != nil {
		return nil, fmt.Errorf("update requires write permission in %s; move Dipper to a writable location or upgrade through the system installer: %w", root, err)
	}
	success := false
	defer func() {
		if !success {
			os.RemoveAll(stage)
		}
	}()
	targetHash, err := fileHash(target)
	if err != nil {
		return nil, err
	}
	installHash := targetHash
	if mode == ModeMacOS {
		installHash, err = BundleDigest(installTarget)
		if err != nil {
			return nil, err
		}
	}
	created, err := processIdentity(os.Getpid())
	if err != nil {
		return nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	cwd, err = filepath.EvalSymlinks(cwd)
	if err != nil {
		return nil, err
	}
	control, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	defer control.Close()
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	readyToken, err := randomToken()
	if err != nil {
		return nil, err
	}
	state := helperState{
		Mode: mode, ParentPID: os.Getpid(), ParentCreated: created,
		Target: target, TargetHash: targetHash, InstallTarget: installTarget, InstallHash: installHash,
		Stage: stage, Staged: filepath.Join(stage, "new.exe"), StagedHash: downloaded.SHA256,
		Args: PrivateArguments(args), WorkingDirectory: cwd, Created: time.Now(),
		ControlAddress: control.Addr().String(), ControlToken: token, ReadyToken: readyToken,
	}
	switch mode {
	case ModeMacOS:
		state.Staged, state.StagedHash = filepath.Join(stage, "new.app"), downloaded.BundleSHA256
		err = copyBundle(downloaded.Bundle, state.Staged)
	case ModeDebian:
		state.Staged, state.StagedHash = filepath.Join(stage, "package.deb"), downloaded.PackageSHA256
		state.PackageHash = downloaded.PackageSHA256
		err = copyExclusive(downloaded.Package, state.Staged)
	default:
		err = copyExclusive(downloaded.Executable, state.Staged)
		if err == nil {
			var info os.FileInfo
			info, err = os.Stat(downloaded.Executable)
			if err == nil {
				err = os.Chmod(state.Staged, info.Mode().Perm())
			}
		}
	}
	if err != nil {
		return nil, err
	}
	helper := filepath.Join(stage, "helper.exe")
	if err := copyExclusive(target, helper); err != nil {
		return nil, err
	}
	if err := validateUnixState(state, filepath.Join(stage, "state.json"), helper); err != nil {
		return nil, err
	}
	body, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	statePath := filepath.Join(stage, "state.json")
	if err := os.WriteFile(statePath, body, 0600); err != nil {
		return nil, err
	}
	command := exec.Command(helper, helperFlag, statePath)
	command.Dir = cwd
	if err := command.Start(); err != nil {
		return nil, err
	}
	installer := &Installer{command: command, directory: stage}
	defer func() {
		if !success {
			installer.Abort()
		}
	}()
	stop := context.AfterFunc(ctx, func() { control.Close() })
	defer stop()
	control.(*net.TCPListener).SetDeadline(time.Now().Add(15 * time.Second))
	conn, err := acceptToken(control, "ready:"+token, 15*time.Second)
	if err != nil {
		return nil, fmt.Errorf("updater could not acquire the installation lock or start: %w", err)
	}
	installer.conn = conn
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	success = true
	return installer, nil
}
