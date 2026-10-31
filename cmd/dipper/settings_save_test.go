package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"dipper.local/dipper/internal/core"
)

func TestUISettingsSaveTranslatesTrackerError(t *testing.T) {
	kernel, err := core.Open(filepath.Join(t.TempDir(), "dipper.db"), core.Options{DisableScheduler: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kernel.Close() })
	s := uiFixture(t, "settings")
	s.Core = kernel
	s.Ctx = context.Background()
	s.ExtraTrackers = "udp://ok.example.com:1337/announce\nftp://example.com/announce"
	s.save()
	if strings.Contains(s.SaveError, "invalid tracker") {
		t.Fatalf("raw English validation error reached the UI: %q", s.SaveError)
	}
	if !strings.Contains(s.SaveError, "无法识别的 tracker") || !strings.Contains(s.SaveError, "ftp://example.com/announce") {
		t.Fatalf("save error did not name the bad tracker in Chinese: %q", s.SaveError)
	}
}
