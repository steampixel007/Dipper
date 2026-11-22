package downloader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/anacrolix/torrent"
)

func TestEngineUsesOnlySourceAndConfiguredTrackers(t *testing.T) {
	f := newFixture(t, 128<<10)
	source := []string{"udp://source.example:1337/announce"}
	f.mi.AnnounceList = [][]string{source}
	remote := []string{"https://recommended.example/announce"}
	want := [][]string{source, remote}
	var b bytes.Buffer
	if err := f.mi.Write(&b); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(b.Bytes()) }))
	defer server.Close()
	root := t.TempDir()
	e := testEngine(t, Config{DataDir: filepath.Join(root, "state"), RemoteTrackers: remote})
	task := Task{ID: "url", TorrentURL: server.URL, Folder: filepath.Join(root, "data")}
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	j := e.tasks[task.ID]
	if j == nil || j.torrent == nil {
		e.mu.Unlock()
		t.Fatal("admitted task has no torrent")
	}
	announce := j.torrent.Metainfo().AnnounceList
	e.mu.Unlock()
	if !slices.EqualFunc(announce, want, slices.Equal[[]string]) {
		t.Fatalf("running torrent uses %v, want only %v", announce, want)
	}
	data, err := os.ReadFile(e.metadataPath(task.ID))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := decodeSpec(data)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.EqualFunc(saved.Trackers, want, slices.Equal[[]string]) {
		t.Fatalf("cached metadata uses %v, want only %v", saved.Trackers, want)
	}
}

func hasAll(tiers [][]string, want []string) bool {
	seen := map[string]bool{}
	for _, tier := range tiers {
		for _, tracker := range tier {
			seen[tracker] = true
		}
	}
	for _, tracker := range want {
		if !seen[tracker] {
			return false
		}
	}
	return true
}

func TestEngineMergesExtraTrackersIntoTorrentAndMetadata(t *testing.T) {
	f := newFixture(t, 128<<10)
	var b bytes.Buffer
	if err := f.mi.Write(&b); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(b.Bytes()) }))
	defer server.Close()
	root := t.TempDir()
	extra := []string{"udp://extra.example.com:1337/announce"}
	e := testEngine(t, Config{DataDir: filepath.Join(root, "state"), ExtraTrackers: extra})
	task := Task{ID: "url", TorrentURL: server.URL, Folder: filepath.Join(root, "data")}
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	j := e.tasks[task.ID]
	if j == nil || j.torrent == nil {
		e.mu.Unlock()
		t.Fatal("admitted task has no torrent")
	}
	announce := j.torrent.Metainfo().AnnounceList
	e.mu.Unlock()
	if !hasAll(announce, extra) {
		t.Fatalf("engine torrent lacks extra trackers: %v", announce)
	}
	data, err := os.ReadFile(e.metadataPath(task.ID))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := decodeSpec(data)
	if err != nil {
		t.Fatal(err)
	}
	if !hasAll(saved.Trackers, extra) {
		t.Fatalf("persisted metadata lacks extra trackers: %v", saved.Trackers)
	}
}

func TestSetExtraTrackersAnnouncesToRunningTask(t *testing.T) {
	f := newFixture(t, 128<<10)
	var b bytes.Buffer
	if err := f.mi.Write(&b); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(b.Bytes()) }))
	defer server.Close()
	root := t.TempDir()
	e := testEngine(t, Config{DataDir: filepath.Join(root, "state")})
	task := Task{ID: "url", TorrentURL: server.URL, Folder: filepath.Join(root, "data")}
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	late := "udp://late.example.com:1337/announce"
	e.SetTrackerSettings([]string{late, late}, nil)
	e.mu.Lock()
	j := e.tasks[task.ID]
	if j == nil || j.torrent == nil {
		e.mu.Unlock()
		t.Fatal("admitted task has no torrent")
	}
	announce := j.torrent.Metainfo().AnnounceList
	e.mu.Unlock()
	if !hasAll(announce, []string{late}) {
		t.Fatalf("running torrent lacks the late tracker: %v", announce)
	}
}

func TestSetExtraTrackersReconcilesRemovals(t *testing.T) {
	f := newFixture(t, 128<<10)
	var b bytes.Buffer
	if err := f.mi.Write(&b); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(b.Bytes()) }))
	defer server.Close()
	root := t.TempDir()
	e := testEngine(t, Config{DataDir: filepath.Join(root, "state")})
	task := Task{ID: "url", TorrentURL: server.URL, Folder: filepath.Join(root, "data")}
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	if err := e.saveMetadataLocked(e.tasks[task.ID]); err != nil {
		e.mu.Unlock()
		t.Fatal(err)
	}
	e.mu.Unlock()
	keep := "udp://keep.example.com:1337/announce"
	drop := "udp://drop.example.com:1337/announce"
	e.SetTrackerSettings([]string{keep, drop}, nil)
	e.SetTrackerSettings([]string{keep}, nil)
	e.mu.Lock()
	j := e.tasks[task.ID]
	if j == nil || j.torrent == nil {
		e.mu.Unlock()
		t.Fatal("admitted task has no torrent")
	}
	announce := j.torrent.Metainfo().AnnounceList
	if err := e.saveMetadataLocked(j); err != nil {
		e.mu.Unlock()
		t.Fatal(err)
	}
	e.mu.Unlock()
	if hasAll(announce, []string{drop}) {
		t.Fatalf("removed tracker still announced: %v", announce)
	}
	if !hasAll(announce, []string{keep}) {
		t.Fatalf("reconciled list lost the kept tracker: %v", announce)
	}
	data, err := os.ReadFile(e.metadataPath(task.ID))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := decodeSpec(data)
	if err != nil {
		t.Fatal(err)
	}
	if hasAll(saved.Trackers, []string{drop}) {
		t.Fatalf("persisted metadata still carries the removed tracker: %v", saved.Trackers)
	}
	if !hasAll(saved.Trackers, []string{keep}) {
		t.Fatalf("persisted metadata lost the kept tracker: %v", saved.Trackers)
	}
}

func TestSetExtraTrackersKeepsSourceTrackers(t *testing.T) {
	f := newFixture(t, 128<<10)
	shared := "udp://source.example.com:1337/announce"
	f.mi.AnnounceList = [][]string{{shared}}
	var b bytes.Buffer
	if err := f.mi.Write(&b); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(b.Bytes()) }))
	defer server.Close()
	root := t.TempDir()
	e := testEngine(t, Config{DataDir: filepath.Join(root, "state")})
	task := Task{ID: "url", TorrentURL: server.URL, Folder: filepath.Join(root, "data")}
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	e.SetTrackerSettings([]string{shared}, nil)
	e.SetTrackerSettings(nil, nil)
	e.mu.Lock()
	j := e.tasks[task.ID]
	if j == nil || j.torrent == nil {
		e.mu.Unlock()
		t.Fatal("admitted task has no torrent")
	}
	announce := j.torrent.Metainfo().AnnounceList
	e.mu.Unlock()
	if !hasAll(announce, []string{shared}) {
		t.Fatalf("clearing settings removed the torrent's own tracker: %v", announce)
	}
}

func TestAdmissionExtrasClearOnRunningTasks(t *testing.T) {
	f := newFixture(t, 128<<10)
	var b bytes.Buffer
	if err := f.mi.Write(&b); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(b.Bytes()) }))
	defer server.Close()
	root := t.TempDir()
	extra := "udp://admission.example.com:1337/announce"
	e := testEngine(t, Config{DataDir: filepath.Join(root, "state"), ExtraTrackers: []string{extra}})
	task := Task{ID: "url", TorrentURL: server.URL, Folder: filepath.Join(root, "data")}
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	e.SetTrackerSettings(nil, nil)
	e.mu.Lock()
	j := e.tasks[task.ID]
	if j == nil || j.torrent == nil {
		e.mu.Unlock()
		t.Fatal("admitted task has no torrent")
	}
	announce := j.torrent.Metainfo().AnnounceList
	e.mu.Unlock()
	if len(announce) != 0 {
		t.Fatalf("clearing admission extras left trackers announced: %v", announce)
	}
}

func TestResumedTaskDropsClearedExtraTrackers(t *testing.T) {
	f := newFixture(t, 128<<10)
	var b bytes.Buffer
	if err := f.mi.Write(&b); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(b.Bytes()) }))
	defer server.Close()
	root := t.TempDir()
	extra := "udp://resume.example.com:1337/announce"
	e := testEngine(t, Config{DataDir: filepath.Join(root, "state"), ExtraTrackers: []string{extra}})
	task := Task{ID: "url", TorrentURL: server.URL, Folder: filepath.Join(root, "data")}
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	if err := e.Pause(task.ID); err != nil {
		t.Fatal(err)
	}
	e.SetTrackerSettings(nil, nil)
	if err := e.Resume(task.ID); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	j := e.tasks[task.ID]
	if j == nil || j.torrent == nil {
		e.mu.Unlock()
		t.Fatal("resumed task has no torrent")
	}
	announce := j.torrent.Metainfo().AnnounceList
	e.mu.Unlock()
	if len(announce) != 0 {
		t.Fatalf("resumed task still announces trackers: %v", announce)
	}
}

func TestRestartedEngineDropsClearedExtraTrackers(t *testing.T) {
	f := newFixture(t, 128<<10)
	var b bytes.Buffer
	if err := f.mi.Write(&b); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(b.Bytes()) }))
	defer server.Close()
	root := t.TempDir()
	extra := "udp://restart.example.com:1337/announce"
	e := testEngine(t, Config{DataDir: filepath.Join(root, "state"), ExtraTrackers: []string{extra}})
	task := Task{ID: "url", TorrentURL: server.URL, Folder: filepath.Join(root, "data")}
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	restarted := testEngine(t, Config{DataDir: filepath.Join(root, "state")})
	restarted.mu.Lock()
	j := restarted.tasks[task.ID]
	if j == nil || j.torrent == nil {
		restarted.mu.Unlock()
		t.Fatal("restarted task has no torrent")
	}
	announce := j.torrent.Metainfo().AnnounceList
	restarted.mu.Unlock()
	if len(announce) != 0 {
		t.Fatalf("restarted task still announces trackers: %v", announce)
	}
}

func TestEngineRestoreDropsLegacyBuiltinTrackers(t *testing.T) {
	for _, version := range []int{1, 2, 3} {
		t.Run(fmt.Sprintf("v%d", version), func(t *testing.T) {
			fixture := newFixture(t, 128<<10)
			root := t.TempDir()
			stateDir := filepath.Join(root, "state")
			if err := os.Mkdir(stateDir, 0o755); err != nil {
				t.Fatal(err)
			}
			task := savedTask{Task: Task{ID: "cached", Folder: filepath.Join(root, "data")}}
			source := []string{"udp://source.example:1337/announce"}
			legacy := slices.Clone(legacyBuiltinTrackers)
			if version == 3 {
				source = append(source, legacy[0])
				legacy = legacy[1:]
				task.AppliedBuiltin = legacy
			}
			extra := []string{legacy[1]}
			recommended := []string{legacy[2]}
			fixture.mi.AnnounceList = [][]string{source, legacy}
			var metadata bytes.Buffer
			if err := fixture.mi.Write(&metadata); err != nil {
				t.Fatal(err)
			}
			metadataPath := (&Engine{dir: stateDir}).metadataPath(task.ID)
			if err := os.WriteFile(metadataPath, metadata.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(checkpoint{Version: version, Tasks: []savedTask{task}})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(stateDir, "tasks.json"), data, 0o644); err != nil {
				t.Fatal(err)
			}
			engine := testEngine(t, Config{DataDir: stateDir, ExtraTrackers: extra, RemoteTrackers: recommended})
			engine.mu.Lock()
			entry := engine.tasks[task.ID]
			if entry == nil || entry.torrent == nil {
				engine.mu.Unlock()
				t.Fatal("cached task was not restored")
			}
			announce := entry.torrent.Metainfo().AnnounceList
			engine.mu.Unlock()
			want := [][]string{source, extra, recommended}
			if !slices.EqualFunc(announce, want, slices.Equal[[]string]) {
				t.Fatalf("restored torrent uses %v, want only %v", announce, want)
			}
			data, err = os.ReadFile(metadataPath)
			if err != nil {
				t.Fatal(err)
			}
			saved, err := decodeSpec(data)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.EqualFunc(saved.Trackers, want, slices.Equal[[]string]) {
				t.Fatalf("rewritten metadata uses %v, want only %v", saved.Trackers, want)
			}
		})
	}
}

func TestReconciledTiersOrdersSourceUserAndRemote(t *testing.T) {
	own := "udp://own.example.com:1/announce"
	user := "udp://user.example.com:1/announce"
	remote := "udp://remote.example.com:1/announce"
	tiers, injected := reconciledTiers(
		[][]string{{own}},
		trackerTiers{},
		trackerTiers{user: []string{user}, remote: []string{remote}},
		nil,
	)
	if len(tiers) != 3 ||
		len(tiers[0]) != 1 || tiers[0][0] != own ||
		len(tiers[1]) != 1 || tiers[1][0] != user ||
		len(tiers[2]) != 1 || tiers[2][0] != remote {
		t.Fatalf("tiers must order source, user, then remote: %v", tiers)
	}
	if len(injected.user) != 1 || injected.user[0] != user || len(injected.remote) != 1 || injected.remote[0] != remote {
		t.Fatalf("injected user = %v, injected remote = %v", injected.user, injected.remote)
	}
	shared := "udp://shared.example.com:1/announce"
	tiers, injected = reconciledTiers(nil, trackerTiers{}, trackerTiers{user: []string{shared}, remote: []string{shared}}, nil)
	if len(tiers) != 1 || len(tiers[0]) != 1 || tiers[0][0] != shared || len(injected.remote) != 0 {
		t.Fatalf("a URL both tiers configure belongs to the user tier only: %v %v", tiers, injected.remote)
	}
}

func TestDisabledTrackerStaysOutAcrossRefreshAndRestart(t *testing.T) {
	f := newFixture(t, 128<<10)
	shared := "udp://shared.example:1337/announce"
	sourceOwned := "udp://source.example:1337/announce"
	f.mi.AnnounceList = [][]string{{sourceOwned}}
	var metadata bytes.Buffer
	if err := f.mi.Write(&metadata); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(metadata.Bytes()) }))
	defer server.Close()
	root := t.TempDir()
	config := Config{DataDir: filepath.Join(root, "state"), ExtraTrackers: []string{shared}, RemoteTrackers: []string{shared}}
	engine := testEngine(t, config)
	task := Task{ID: "url", TorrentURL: server.URL, Folder: filepath.Join(root, "data")}
	if err := engine.Ensure(task); err != nil {
		t.Fatal(err)
	}
	checkTrackers := func(engine *Engine, stage string, wantShared bool) {
		t.Helper()
		engine.mu.Lock()
		defer engine.mu.Unlock()
		entry := engine.tasks[task.ID]
		if entry == nil || entry.torrent == nil {
			t.Fatalf("%s: no running torrent", stage)
		}
		tiers := entry.torrent.Metainfo().AnnounceList
		if hasAll(tiers, []string{shared}) != wantShared || !hasAll(tiers, []string{sourceOwned}) {
			t.Fatalf("%s: expected shared=%t and preserved source address, got %v", stage, wantShared, tiers)
		}
	}
	checkTrackers(engine, "admission", true)
	config.DisabledTrackers = []string{shared, sourceOwned}
	engine.SetTrackerSettings(config.ExtraTrackers, config.DisabledTrackers)
	checkTrackers(engine, "disable", false)
	engine.SetRemoteTrackers(config.RemoteTrackers)
	checkTrackers(engine, "remote refresh", false)
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	restarted := testEngine(t, config)
	checkTrackers(restarted, "restart", false)
	restarted.SetTrackerSettings(config.ExtraTrackers, []string{sourceOwned})
	checkTrackers(restarted, "re-enable", true)
	config.DataDir = filepath.Join(root, "fresh-state")
	fresh := testEngine(t, config)
	task.Folder = filepath.Join(root, "fresh-data")
	if err := fresh.Ensure(task); err != nil {
		t.Fatal(err)
	}
	checkTrackers(fresh, "new task with exclusions", false)
}

func TestTrackerRestartStopsHTTPAnnouncesToDisabledAddress(t *testing.T) {
	fixture := newFixture(t, 128<<10)
	var metadata bytes.Buffer
	requested := make(chan string, 16)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/torrent" {
			writer.Write(metadata.Bytes())
			return
		}
		if request.URL.Query().Get("event") != "stopped" {
			select {
			case requested <- request.URL.Path:
			default:
			}
		}
		writer.Write([]byte("d8:intervali60e5:peers0:e"))
	}))
	defer server.Close()
	own, extra := server.URL+"/own", server.URL+"/extra"
	fixture.mi.AnnounceList = [][]string{{own}}
	if err := fixture.mi.Write(&metadata); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	config := Config{
		DataDir: filepath.Join(root, "state"), ExtraTrackers: []string{extra},
	}
	open := func() *Engine {
		engine, err := newEngine(config, func(client *torrent.ClientConfig) {
			localConfig(client)
			client.DisableTrackers = false
			client.HTTPProxy = nil
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = engine.Close() })
		engine.SetNetworkAvailable(true)
		return engine
	}
	waitFor := func(path string) {
		t.Helper()
		timeout := time.NewTimer(5 * time.Second)
		defer timeout.Stop()
		for {
			select {
			case got := <-requested:
				if got == path {
					return
				}
			case <-timeout.C:
				t.Fatalf("no HTTP announce to %s", path)
			}
		}
	}
	engine := open()
	task := Task{ID: "url", TorrentURL: server.URL + "/torrent", Folder: filepath.Join(root, "data")}
	if err := engine.Ensure(task); err != nil {
		t.Fatal(err)
	}
	waitFor("/extra")
	config.DisabledTrackers = append(config.DisabledTrackers, extra)
	engine.SetTrackerSettings(config.ExtraTrackers, config.DisabledTrackers)
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	for len(requested) > 0 {
		<-requested
	}
	reopened := open()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	for expectOwn := true; expectOwn; {
		select {
		case got := <-requested:
			if got == "/extra" {
				t.Fatal("restarted torrent announced to the disabled address")
			}
			expectOwn = got != "/own"
		case <-timeout.C:
			t.Fatal("no HTTP announce to the torrent's own address")
		}
	}
	grace := time.NewTimer(500 * time.Millisecond)
	defer grace.Stop()
	select {
	case got := <-requested:
		if got == "/extra" {
			t.Fatal("restarted torrent announced to the disabled address")
		}
	case <-grace.C:
	}
	reopened.SetTrackerSettings(config.ExtraTrackers, nil)
	waitFor("/extra")
}

func TestEngineMergesRemoteTrackersIntoTorrentAndMetadata(t *testing.T) {
	f := newFixture(t, 128<<10)
	var b bytes.Buffer
	if err := f.mi.Write(&b); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(b.Bytes()) }))
	defer server.Close()
	root := t.TempDir()
	remote := []string{"udp://remote.example.com:1337/announce"}
	e := testEngine(t, Config{DataDir: filepath.Join(root, "state"), RemoteTrackers: remote})
	task := Task{ID: "url", TorrentURL: server.URL, Folder: filepath.Join(root, "data")}
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	j := e.tasks[task.ID]
	if j == nil || j.torrent == nil {
		e.mu.Unlock()
		t.Fatal("admitted task has no torrent")
	}
	announce := j.torrent.Metainfo().AnnounceList
	e.mu.Unlock()
	if !hasAll(announce, remote) {
		t.Fatalf("engine torrent lacks remote trackers: %v", announce)
	}
	data, err := os.ReadFile(e.metadataPath(task.ID))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := decodeSpec(data)
	if err != nil {
		t.Fatal(err)
	}
	if !hasAll(saved.Trackers, remote) {
		t.Fatalf("persisted metadata lacks remote trackers: %v", saved.Trackers)
	}
}

func TestSetRemoteTrackersAnnouncesToRunningTaskAndClears(t *testing.T) {
	f := newFixture(t, 128<<10)
	var b bytes.Buffer
	if err := f.mi.Write(&b); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(b.Bytes()) }))
	defer server.Close()
	root := t.TempDir()
	e := testEngine(t, Config{DataDir: filepath.Join(root, "state")})
	task := Task{ID: "url", TorrentURL: server.URL, Folder: filepath.Join(root, "data")}
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	remote := "udp://late.example.com:1337/announce"
	e.SetRemoteTrackers([]string{remote, remote})
	e.mu.Lock()
	j := e.tasks[task.ID]
	if j == nil || j.torrent == nil {
		e.mu.Unlock()
		t.Fatal("admitted task has no torrent")
	}
	announce := j.torrent.Metainfo().AnnounceList
	e.mu.Unlock()
	if !hasAll(announce, []string{remote}) {
		t.Fatalf("a late refresh must announce to the running task: %v", announce)
	}
	e.SetRemoteTrackers(nil)
	e.mu.Lock()
	announce = j.torrent.Metainfo().AnnounceList
	e.mu.Unlock()
	if len(announce) != 0 {
		t.Fatalf("clearing the remote tier left trackers announced: %v", announce)
	}
}

func TestRestartedEngineDropsClearedRemoteTrackers(t *testing.T) {
	f := newFixture(t, 128<<10)
	var b bytes.Buffer
	if err := f.mi.Write(&b); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(b.Bytes()) }))
	defer server.Close()
	root := t.TempDir()
	remote := "udp://restart.example.com:1337/announce"
	e := testEngine(t, Config{DataDir: filepath.Join(root, "state"), RemoteTrackers: []string{remote}})
	task := Task{ID: "url", TorrentURL: server.URL, Folder: filepath.Join(root, "data")}
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	restarted := testEngine(t, Config{DataDir: filepath.Join(root, "state")})
	restarted.mu.Lock()
	j := restarted.tasks[task.ID]
	if j == nil || j.torrent == nil {
		restarted.mu.Unlock()
		t.Fatal("restarted task has no torrent")
	}
	announce := j.torrent.Metainfo().AnnounceList
	restarted.mu.Unlock()
	if len(announce) != 0 {
		t.Fatalf("restarted task still announces trackers: %v", announce)
	}
}

func TestAdmissionKeepsSourceTrackersUnderConfiguredExtras(t *testing.T) {
	f := newFixture(t, 128<<10)
	shared := "udp://source.example.com:1337/announce"
	f.mi.AnnounceList = [][]string{{shared}}
	var b bytes.Buffer
	if err := f.mi.Write(&b); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(b.Bytes()) }))
	defer server.Close()
	root := t.TempDir()
	e := testEngine(t, Config{DataDir: filepath.Join(root, "state"), ExtraTrackers: []string{shared}})
	task := Task{ID: "url", TorrentURL: server.URL, Folder: filepath.Join(root, "data")}
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	e.SetTrackerSettings(nil, nil)
	e.mu.Lock()
	j := e.tasks[task.ID]
	if j == nil || j.torrent == nil {
		e.mu.Unlock()
		t.Fatal("admitted task has no torrent")
	}
	announce := j.torrent.Metainfo().AnnounceList
	e.mu.Unlock()
	if !hasAll(announce, []string{shared}) {
		t.Fatalf("clearing settings removed the torrent's own tracker: %v", announce)
	}
}
