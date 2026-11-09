package update

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPackageOwnerParsingRejectsExtraPathsAndFields(t *testing.T) {
	target := "/opt/dipper/dipper"
	if _, err := parsePackageOwner("dipper: "+target+" /unexpected", target); err == nil {
		t.Fatal("ambiguous dpkg ownership was accepted")
	}
	if owner, err := parsePackageOwner("dipper: "+target, target); err != nil || owner != "dipper" {
		t.Fatalf("exact ownership rejected: %q %v", owner, err)
	}
}

func TestDebianUpgradeScriptUsesOnlyPositionalValidatedPackage(t *testing.T) {
	markers := []string{
		`/usr/bin/sha256sum -- "$stage/update.deb"`,
		`[ "${actual%% *}" = "$2" ]`,
		`[ "$(/usr/bin/dpkg-deb -f "$stage/update.deb" Package)" = dipper ]`,
		`/usr/bin/dpkg --install "$stage/update.deb"`,
	}
	for _, marker := range markers {
		if !strings.Contains(debianUpgradeScript, marker) {
			t.Fatalf("fixed package script lacks validation: %s", marker)
		}
	}
	for _, unsafe := range []string{"$0", "eval ", "sh -c", "PKG=", "dpkg -i"} {
		if strings.Contains(debianUpgradeScript, unsafe) {
			t.Fatalf("fixed package script accepts dynamic instruction: %s", unsafe)
		}
	}
}

func TestUpgradeSuccessRequiresReadyRestartedApplication(t *testing.T) {
	state := replacementFixture(t)
	state.Mode, state.PackageHash = ModeDebian, state.StagedHash
	err := upgradeAndRestart(state, 100*time.Millisecond, func(string, []string, string) (restartedProcess, error) {
		return restartedProcess{exited: make(chan error)}, nil
	}, func(helperState) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "without readiness") {
		t.Fatalf("unacknowledged restart reported package success: %v", err)
	}
	if _, err := os.Stat(filepath.Join(state.Stage, "old.exe")); !os.IsNotExist(err) {
		t.Fatal("package update created executable backup")
	}
}

func TestUpgradeCancellationKeepsOriginalArguments(t *testing.T) {
	state := replacementFixture(t)
	state.Mode, state.PackageHash = ModeDebian, state.StagedHash
	finished := make(chan error, 1)
	denied := errors.New("authorization cancelled")
	err := upgradeAndRestart(state, time.Second, func(_ string, args []string, _ string) (restartedProcess, error) {
		if !reflect.DeepEqual(PrivateArguments(args), PrivateArguments(state.Args)) {
			t.Errorf("authorization recovery changed arguments: %v", args)
		}
		go acknowledgeFixture(args, finished)
		return restartedProcess{}, nil
	}, func(helperState) error { return denied })
	if finishedErr := <-finished; !errors.Is(err, denied) || finishedErr != nil {
		t.Fatalf("cancellation outcome lost: %v", err)
	}
}
