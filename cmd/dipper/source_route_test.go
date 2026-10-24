package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dipper.local/dipper/internal/source"
	"github.com/egoist/mygo/ui"
)

const routeCatalogMarkup = `<div class="sk-bangumi" data-dayofweek="1"><ul class="an-ul"><li><span data-bangumiid="1" data-src="/cover.png"></span><a class="an-text" href="/Home/Bangumi/1">节目</a></li></ul></div>`

func routeFixture(t *testing.T, page string) *State {
	t.Helper()
	s := uiFixture(t, page)
	s.SourceRoutes = []sourceRoute{
		{Label: "线路一 · 测试（默认）", Origin: "https://route-a.test"},
		{Label: "线路二 · 测试", Origin: "https://route-b.test"},
	}
	s.SourceRoute = s.SourceRoutes[0].Label
	return s
}

func TestSourceRouteSwitchTestsThenSaves(t *testing.T) {
	s := routeFixture(t, "settings")
	initial := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(routeCatalogMarkup)) }))
	defer initial.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(routeCatalogMarkup)) }))
	defer good.Close()
	s.Source = source.New(initial.URL)
	s.SourceRoutes[0].Origin = initial.URL
	s.SourceRoutes[1].Origin = good.URL
	s.Ctx = t.Context()
	second := s.SourceRoutes[1]
	s.switchSourceRoute(second.Label)
	deadline := time.Now().Add(2 * time.Second)
	for s.SourceTesting && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if s.SourceTesting {
		t.Fatal("route switch did not finish")
	}
	if s.SourceRoute != second.Label || s.Source.Origin() != second.Origin {
		t.Fatalf("route = %q origin = %q", s.SourceRoute, s.Source.Origin())
	}
	if s.SourceRouteError != "" || s.Notice != "已切换线路，现有追番不受影响。" {
		t.Fatalf("error = %q notice = %q", s.SourceRouteError, s.Notice)
	}
}

func TestSourceRouteFailureRollsBackAndKeepsChoice(t *testing.T) {
	s := routeFixture(t, "settings")
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer bad.Close()
	s.Source = source.New(bad.URL)
	s.SourceRoutes[1].Origin = bad.URL
	s.Ctx = t.Context()
	previousLabel, previousOrigin := s.SourceRoute, s.Source.Origin()
	second := s.SourceRoutes[1]
	s.switchSourceRoute(second.Label)
	deadline := time.Now().Add(2 * time.Second)
	for s.SourceTesting && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if s.SourceTesting {
		t.Fatal("route switch did not finish")
	}
	if s.SourceRoute != previousLabel || s.Source.Origin() != previousOrigin {
		t.Fatalf("failed switch changed state: route = %q origin = %q", s.SourceRoute, s.Source.Origin())
	}
	if s.SourceRouteError != "这条线路返回 HTTP 503，换一条线路试试。" {
		t.Fatalf("error = %q", s.SourceRouteError)
	}
}

func TestSourceRoutePreferenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "preferences.json")
	if err := os.WriteFile(path, []byte(`{"SourceOrigin":"https://mikanani.me"}`), 0644); err != nil {
		t.Fatal(err)
	}
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(routeCatalogMarkup)) }))
	defer good.Close()
	s := newState(dir)
	s.SourceRoutes = []sourceRoute{{Label: "线路一", Origin: "https://mikanime.tv"}, {Label: "线路二", Origin: "https://mikanani.me"}}
	s.SourceRoute = s.SourceRoutes[0].Label
	s.Source = source.New(good.URL)
	// Retarget to the saved origin, then save and reload like startup does.
	var pref preferences
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &pref); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Source.Retarget(pref.SourceOrigin); err != nil {
		t.Fatal(err)
	}
	s.SourceRoute = s.sourceLabel(pref.SourceOrigin)
	if err := s.savePreferences(); err != nil {
		t.Fatal(err)
	}
	var saved preferences
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.SourceOrigin != "https://mikanani.me" {
		t.Fatalf("saved origin = %q", saved.SourceOrigin)
	}
}

func TestUISettingsSourceRouteDisabledWhileBusy(t *testing.T) {
	s := routeFixture(t, "settings")
	s.Busy = true
	tt := ui.NewTester(s.view, 1320, 860)
	// The dropdown is disabled while busy; a click on its label cannot select.
	if err := tt.Click(s.SourceRoutes[1].Label); err == nil {
		t.Fatal("busy settings allowed selecting a source route")
	}
	if s.SourceRoute != s.SourceRoutes[0].Label {
		t.Fatalf("busy settings changed route to %q", s.SourceRoute)
	}
}
