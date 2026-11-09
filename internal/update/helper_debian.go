package update

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const debianUpgradeScript = `set -eu
PATH=/usr/sbin:/usr/bin:/sbin:/bin
export PATH
umask 077
stage=$(/usr/bin/mktemp -d /var/tmp/dipper-upgrade.XXXXXXXX)
trap '/bin/rm -rf -- "$stage"' EXIT HUP INT TERM
/bin/cp -- "$1" "$stage/update.deb"
actual=$(/usr/bin/sha256sum -- "$stage/update.deb")
[ "${actual%% *}" = "$2" ]
[ "$(/usr/bin/dpkg-deb -f "$stage/update.deb" Package)" = dipper ]
installed=$(/usr/bin/dpkg-query -W -f='${Version}' dipper)
incoming=$(/usr/bin/dpkg-deb -f "$stage/update.deb" Version)
/usr/bin/dpkg --compare-versions "$incoming" gt "$installed"
/usr/bin/dpkg --install "$stage/update.deb"
`

func parsePackageOwner(body, target string) (string, error) {
	line := strings.TrimSpace(body)
	owner, path, found := strings.Cut(line, ": ")
	if !found || path != target || owner == "" || strings.ContainsAny(owner, "\r\n, ") {
		return "", errors.New("ambiguous Debian package ownership; refusing direct executable replacement")
	}
	return owner, nil
}

func linuxPackageOwner(ctx context.Context, target string) (string, error) {
	if runtime.GOOS != "linux" {
		return "", nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	body, err := exec.CommandContext(ctx, "/usr/bin/dpkg-query", "--search", target).Output()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return "", nil
		}
		return "", fmt.Errorf("cannot verify package ownership: %w", err)
	}
	return parsePackageOwner(string(body), target)
}

func authorizedPackageUpgrade(state helperState) error {
	if runtime.GOOS != "linux" {
		return errors.New("Debian updates require Linux")
	}
	if err := hashMatches(state.Staged, state.PackageHash); err != nil {
		return err
	}
	command := exec.Command("/usr/bin/pkexec", "/bin/sh", "-c", debianUpgradeScript,
		"dipper-update", state.Staged, state.PackageHash)
	body, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("system package upgrade failed or authorization was cancelled: %w: %s", err, strings.TrimSpace(string(body)))
	}
	return nil
}

func installDebianAndRestart(state helperState, timeout time.Duration, start restartFunc) error {
	return upgradeAndRestart(state, timeout, start, authorizedPackageUpgrade)
}

func upgradeAndRestart(state helperState, timeout time.Duration, start restartFunc, upgrade func(helperState) error) error {
	if err := hashMatches(state.Target, state.TargetHash); err != nil {
		return err
	}
	if err := hashMatches(state.Staged, state.PackageHash); err != nil {
		return err
	}
	upgradeErr := upgrade(state)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return errors.Join(upgradeErr, err)
	}
	defer listener.Close()
	listener.(*net.TCPListener).SetDeadline(time.Now().Add(timeout))
	restore := func(cause error) error {
		return errors.Join(upgradeErr, cause, errors.New("Debian package state requires repair through the system package manager; user data was not reverted"))
	}
	complete := func() {
		os.Remove(state.Staged)
		os.Remove(filepath.Join(state.Stage, "state.json"))
		markStageComplete(state.Stage)
	}
	err = restartWithReadiness(state, listener, timeout, start, restore, complete, "use the system package manager for Debian package recovery")
	return errors.Join(upgradeErr, err)
}
