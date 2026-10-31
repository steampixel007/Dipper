package core

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseTrackersSplitsAndDeduplicates(t *testing.T) {
	text := "udp://a.example.com:1337/announce\nhttp://b.example.com/announce, ; udp://a.example.com:1337/announce"
	want := []string{"udp://a.example.com:1337/announce", "http://b.example.com/announce"}
	if got := ParseTrackers(text); !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseTrackers = %v, want %v", got, want)
	}
}

func TestParseTrackersDropsUnparsableEntries(t *testing.T) {
	if got := ParseTrackers("not a url\nudp://ok.example.com:1/announce"); !reflect.DeepEqual(got, []string{"udp://ok.example.com:1/announce"}) {
		t.Fatalf("ParseTrackers = %v", got)
	}
	if got := ParseTrackers("  "); got != nil {
		t.Fatalf("ParseTrackers = %v, want nil", got)
	}
}

func TestSeedRatioTargetMapsLabels(t *testing.T) {
	cases := map[string]float64{"": 1, "1:1": 1, "2:1": 2, "3:1": 3, "不限": -1, "未知": 1}
	for label, want := range cases {
		if got := SeedRatioTarget(label); got != want {
			t.Errorf("SeedRatioTarget(%q) = %v, want %v", label, got, want)
		}
	}
}

func TestSettingsValidateSeedRatioAndTrackers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	c := openTest(t, path, Options{})
	defer c.Close()
	settings, err := c.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.SeedRatio = "4:1"
	if err := c.SetSettings(settings); err == nil {
		t.Fatal("unknown seed ratio label was accepted")
	}
	settings.SeedRatio = "2:1"
	settings.ExtraTrackers = "ftp://nope/announce"
	if err := c.SetSettings(settings); err == nil {
		t.Fatal("unparsable tracker line was accepted")
	}
	settings.ExtraTrackers = "udp://a.example.com:1337/announce"
	if err := c.SetSettings(settings); err != nil {
		t.Fatal(err)
	}
	got, err := c.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.SeedRatio != "2:1" || got.ExtraTrackers != "udp://a.example.com:1337/announce" {
		t.Fatalf("settings roundtrip lost seed options: %+v", got)
	}
}

func TestValidateTrackersReportsTheOffendingLine(t *testing.T) {
	err := ValidateTrackers("udp://ok.example.com:1/announce\nftp://nope/announce")
	var bad *TrackerError
	if !errors.As(err, &bad) {
		t.Fatalf("ValidateTrackers = %v, want a TrackerError", err)
	}
	if bad.Line != "ftp://nope/announce" {
		t.Fatalf("TrackerError.Line = %q", bad.Line)
	}
	if err.Error() != `invalid tracker "ftp://nope/announce"` {
		t.Fatalf("ValidateTrackers = %q", err.Error())
	}
}
