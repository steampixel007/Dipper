package update

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUpdateFailureReportSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	stage := filepath.Join(dir, ".dipper-update-failed")
	if err := os.Mkdir(stage, 0700); err != nil {
		t.Fatal(err)
	}
	cause := errors.New("fixture authorization denied")
	if err := persistUpdateFailure(stage, cause); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(stage, failureReportName)
	info, err := os.Stat(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("failure report permissions = %o, want 600", info.Mode().Perm())
	}

	message, err := ConsumeFailureReport(filepath.Join(dir, "Dipper.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(message, helperFailureText) || !strings.Contains(message, cause.Error()) {
		t.Fatalf("failure report lost the helper explanation: %q", message)
	}
	if _, err := os.Stat(reportPath); !os.IsNotExist(err) {
		t.Fatalf("consumed failure report remains: %v", err)
	}
	if _, err := os.Stat(stage); err != nil {
		t.Fatalf("consumption removed the failed stage: %v", err)
	}
}

func TestFailureReportConsumptionRejectsMalformedReports(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "empty", body: ""},
		{name: "malformed", body: "{"},
		{name: "unknown field", body: `{"message":"failure","extra":true}`},
		{name: "trailing data", body: `{"message":"failure"}{}`},
		{name: "oversized", body: strings.Repeat("x", maxFailureReportSize+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			stage := filepath.Join(dir, ".dipper-update-failed")
			if err := os.Mkdir(stage, 0700); err != nil {
				t.Fatal(err)
			}
			reportPath := filepath.Join(stage, failureReportName)
			if err := os.WriteFile(reportPath, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			message, err := ConsumeFailureReport(filepath.Join(dir, "Dipper.exe"))
			if message != "" || err == nil {
				t.Fatalf("malformed failure report was accepted: message=%q err=%v", message, err)
			}
			if _, err := os.Stat(reportPath); err != nil {
				t.Fatalf("malformed failure report was removed: %v", err)
			}
		})
	}
}

func TestFailureReportConsumptionDoesNotFollowLinks(t *testing.T) {
	dir := t.TempDir()
	stage := filepath.Join(dir, ".dipper-update-failed")
	if err := os.Mkdir(stage, 0700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(dir, "outside.json")
	if err := os.WriteFile(outside, []byte(`{"message":"outside failure"}`), 0600); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(stage, failureReportName)
	if err := os.Symlink(outside, reportPath); err != nil {
		t.Skipf("this host cannot create symbolic links: %v", err)
	}

	message, err := ConsumeFailureReport(filepath.Join(dir, "Dipper.exe"))
	if message != "" || err == nil {
		t.Fatalf("linked failure report was accepted: message=%q err=%v", message, err)
	}
	if _, err := os.Lstat(reportPath); err != nil {
		t.Fatalf("linked failure report was removed: %v", err)
	}
	if body, err := os.ReadFile(outside); err != nil || string(body) != `{"message":"outside failure"}` {
		t.Fatalf("linked failure report target changed: %q %v", body, err)
	}
}

func TestCleanupPreservesFailedUpdateStage(t *testing.T) {
	dir := t.TempDir()
	stage := filepath.Join(dir, ".dipper-update-failed")
	for name, body := range map[string]string{
		"helper.exe":      "helper fixture",
		"state.json":      "failed update state",
		failureReportName: `{"message":"update failed"}`,
	} {
		path := filepath.Join(stage, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}

	CleanupCompleted(filepath.Join(dir, "Dipper.exe"))

	if _, err := os.Stat(filepath.Join(stage, "helper.exe")); err != nil {
		t.Fatalf("cleanup removed a failed update stage: %v", err)
	}
}
