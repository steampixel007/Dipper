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

func bundleFixture(t *testing.T) helperState {
	t.Helper()
	state := replacementFixture(t)
	state.Mode = ModeMacOS
	state.InstallTarget = filepath.Join(filepath.Dir(state.Target), "Dipper.app")
	state.Target = filepath.Join(state.InstallTarget, "Contents", "MacOS", "Dipper")
	state.Staged = filepath.Join(state.Stage, "new.app")
	for root, version := range map[string]string{state.InstallTarget: "old", state.Staged: "new"} {
		for relative, body := range map[string]string{
			filepath.Join("Contents", "MacOS", "Dipper"):    version + " executable fixture",
			filepath.Join("Contents", "Resources", "cover"): version + " resource fixture",
			filepath.Join("Contents", "Info.plist"):         version + " metadata fixture",
		} {
			target := filepath.Join(root, relative)
			if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, []byte(body), 0700); err != nil {
				t.Fatal(err)
			}
		}
	}
	state.TargetHash, _ = fileHash(state.Target)
	state.InstallHash = mustBundleDigest(t, state.InstallTarget)
	state.StagedHash = mustBundleDigest(t, state.Staged)
	return state
}

func mustBundleDigest(t *testing.T, root string) string {
	t.Helper()
	digest, err := BundleDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func acknowledgeFixture(args []string, result chan<- error) {
	var address, token string
	for index, arg := range args {
		if arg == "--software-update-ready-address" {
			address = args[index+1]
		}
		if arg == "--software-update-ready-token" {
			token = args[index+1]
		}
	}
	err := AuthorizeRestart(address, token)
	if err == nil {
		err = NotifyReady(address, token)
	}
	result <- err
}

func TestMacReplacementMovesWholeBundle(t *testing.T) {
	state := bundleFixture(t)
	if err := os.Remove(filepath.Join(state.Stage, "new.exe")); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	err := replaceAndRestart(state, 3*time.Second, func(target string, args []string, cwd string) (restartedProcess, error) {
		if target != state.Target || cwd != state.WorkingDirectory ||
			!reflect.DeepEqual(PrivateArguments(args), PrivateArguments(state.Args)) {
			t.Error("bundle replacement lost original arguments or working directory")
		}
		if err := payloadMatches(state.InstallTarget, state.StagedHash, ModeMacOS); err != nil {
			t.Errorf("bundle resources were not replaced together: %v", err)
		}
		go acknowledgeFixture(args, finished)
		return restartedProcess{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"preferences.json", "state.sqlite", "unrelated.txt"} {
		if _, err := os.Stat(filepath.Join(state.WorkingDirectory, name)); err != nil {
			t.Fatalf("user data changed: %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(state.Stage, "old.exe")); !os.IsNotExist(err) {
		t.Fatal("acknowledged replacement retained old bundle")
	}
}

func TestMacReplacementRestoresWholeBundleOnExit(t *testing.T) {
	state := bundleFixture(t)
	exited := make(chan error, 1)
	exited <- errors.New("fixture startup failed")
	err := replaceAndRestart(state, time.Second, func(string, []string, string) (restartedProcess, error) {
		return restartedProcess{exited: exited}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "old executable restored") {
		t.Fatalf("missing bundle recovery: %v", err)
	}
	if err := payloadMatches(state.InstallTarget, state.InstallHash, ModeMacOS); err != nil {
		t.Fatalf("original resources or executable were not restored: %v", err)
	}
}

func TestMacReplacementRejectsResourceTampering(t *testing.T) {
	state := bundleFixture(t)
	path := filepath.Join(state.Staged, "Contents", "Resources", "cover")
	if err := os.WriteFile(path, []byte("tampered"), 0700); err != nil {
		t.Fatal(err)
	}
	called := false
	err := replaceAndRestart(state, time.Second, func(string, []string, string) (restartedProcess, error) {
		called = true
		return restartedProcess{}, nil
	})
	if err == nil || called {
		t.Fatalf("changed resources reached executable replacement: %v", err)
	}
	if err := payloadMatches(state.InstallTarget, state.InstallHash, ModeMacOS); err != nil {
		t.Fatal("tampered staged resources changed original bundle")
	}
}

func TestBundleCopyPreservesResourcesAndDigest(t *testing.T) {
	state := bundleFixture(t)
	target := filepath.Join(t.TempDir(), "copied.app")
	if err := copyBundle(state.InstallTarget, target); err != nil {
		t.Fatal(err)
	}
	if actual := mustBundleDigest(t, target); actual != state.InstallHash {
		t.Fatal("bundle copy lost content or permissions")
	}
}

func TestLinuxReplacementRestoresOriginalAfterFailedStart(t *testing.T) {
	state := replacementFixture(t)
	state.Mode, state.InstallTarget, state.InstallHash = ModeLinux, state.Target, state.TargetHash
	err := replaceAndRestart(state, time.Second, func(string, []string, string) (restartedProcess, error) {
		return restartedProcess{}, errors.New("fixture cannot execute")
	})
	if err == nil {
		t.Fatal("failed Linux restart reported success")
	}
	if err := hashMatches(state.Target, state.TargetHash); err != nil {
		t.Fatalf("failed Linux restart did not restore executable: %v", err)
	}
}

func TestReplacementDoesNotRestoreTamperedBackup(t *testing.T) {
	state := replacementFixture(t)
	err := replaceAndRestart(state, time.Second, func(string, []string, string) (restartedProcess, error) {
		if err := os.WriteFile(filepath.Join(state.Stage, "old.exe"), []byte("tampered"), 0700); err != nil {
			t.Fatal(err)
		}
		return restartedProcess{}, errors.New("fixture cannot execute")
	})
	if err == nil || !strings.Contains(err.Error(), "backup changed") {
		t.Fatalf("tampered backup not rejected: %v", err)
	}
	if err := hashMatches(state.Target, state.StagedHash); err != nil {
		t.Fatal("valid new payload removed despite invalid backup")
	}
}

func TestDebianUpgradeRequiresVerifiedPackageBeforeAuthorization(t *testing.T) {
	state := replacementFixture(t)
	state.PackageHash = state.StagedHash
	if err := os.WriteFile(state.Staged, []byte("changed package"), 0700); err != nil {
		t.Fatal(err)
	}
	called := false
	err := upgradeAndRestart(state, time.Second, nil, func(helperState) error {
		called = true
		return nil
	})
	if err == nil || called {
		t.Fatal("tampered Debian package reached system authorization")
	}
}

func TestDebianAuthorizationFailureRestartsExistingApplication(t *testing.T) {
	state := replacementFixture(t)
	state.Mode, state.PackageHash = ModeDebian, state.StagedHash
	finished := make(chan error, 1)
	denied := errors.New("fixture authorization denied")
	err := upgradeAndRestart(state, time.Second, func(target string, args []string, cwd string) (restartedProcess, error) {
		if target != state.Target || cwd != state.WorkingDirectory ||
			!reflect.DeepEqual(PrivateArguments(args), PrivateArguments(state.Args)) {
			t.Error("Debian recovery lost restart context")
		}
		if err := hashMatches(state.Target, state.TargetHash); err != nil {
			t.Error("authorization denial replaced the installed executable")
		}
		go acknowledgeFixture(args, finished)
		return restartedProcess{}, nil
	}, func(helperState) error { return denied })
	if !errors.Is(err, denied) {
		t.Fatalf("authorization failure was not reported: %v", err)
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
}

func TestDebianOwnershipRejectsAmbiguousResults(t *testing.T) {
	target := "/opt/dipper/dipper"
	for _, body := range []string{"", "dipper: /another/file", "dipper: " + target + "\nother: " + target, "dipper, other: " + target} {
		if _, err := parsePackageOwner(body, target); err == nil {
			t.Errorf("accepted ambiguous package ownership %q", body)
		}
	}
	if owner, err := parsePackageOwner("dipper: "+target+"\n", target); err != nil || owner != "dipper" {
		t.Fatalf("valid package ownership rejected: %q %v", owner, err)
	}
}
