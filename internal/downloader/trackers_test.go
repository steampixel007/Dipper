package downloader

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/anacrolix/torrent"
)

func TestAppendBuiltinTrackersAddsOneTierWithoutDuplicates(t *testing.T) {
	spec := torrent.TorrentSpec{Trackers: [][]string{{"http://127.0.0.1:9/announce"}, {"udp://tracker.opentrackr.org:1337/announce"}}}
	appendBuiltinTrackers(&spec)
	tiers := spec.Trackers
	if len(tiers) != 3 {
		t.Fatalf("expected the existing two tiers plus one built-in tier, got %d: %v", len(tiers), tiers)
	}
	seen := map[string]bool{}
	for _, tier := range tiers {
		for _, tracker := range tier {
			if seen[tracker] {
				t.Fatalf("duplicate tracker %q", tracker)
			}
			seen[tracker] = true
		}
	}
	for _, tracker := range builtinTrackers {
		if !seen[tracker] {
			t.Fatalf("missing built-in tracker %q: %v", tracker, tiers)
		}
	}
}

func TestEngineAppendsBuiltinTrackersToTorrentAndMetadata(t *testing.T) {
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
	j := e.tasks[task.ID]
	if j == nil || j.torrent == nil {
		e.mu.Unlock()
		t.Fatal("admitted task has no torrent")
	}
	announce := j.torrent.Metainfo().AnnounceList
	e.mu.Unlock()
	if !hasAll(announce, builtinTrackers) {
		t.Fatalf("engine torrent lacks built-in trackers: %v", announce)
	}
	data, err := os.ReadFile(e.metadataPath(task.ID))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := decodeSpec(data)
	if err != nil {
		t.Fatal(err)
	}
	if !hasAll(saved.Trackers, builtinTrackers) {
		t.Fatalf("persisted metadata lacks built-in trackers: %v", saved.Trackers)
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

func TestExtraTrackersMergeAsOwnTierBeforeBuiltins(t *testing.T) {
	spec := torrent.TorrentSpec{Trackers: [][]string{{builtinTrackers[0]}}}
	extra := normalizeTrackers([]string{"udp://extra.example.com:1337/announce", "udp://extra.example.com:1337/announce", builtinTrackers[0]})
	appendTrackers(&spec, extra, builtinTrackers)
	tiers := spec.Trackers
	if len(tiers) != 3 || len(tiers[1]) != 1 || tiers[1][0] != "udp://extra.example.com:1337/announce" {
		t.Fatalf("user tier must carry only its own URLs before the built-ins: %v", tiers)
	}
	if !hasAll(tiers, builtinTrackers) {
		t.Fatalf("missing built-in trackers: %v", tiers)
	}
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
	if !hasAll(announce, extra) || !hasAll(announce, builtinTrackers) {
		t.Fatalf("engine torrent lacks extra or built-in trackers: %v", announce)
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
	e.SetExtraTrackers([]string{late, late})
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
	e.SetExtraTrackers([]string{keep, drop})
	e.SetExtraTrackers([]string{keep})
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
	if !hasAll(announce, []string{keep}) || !hasAll(announce, builtinTrackers) {
		t.Fatalf("reconciled list lost kept or builtin trackers: %v", announce)
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
	if !hasAll(saved.Trackers, []string{keep}) || !hasAll(saved.Trackers, builtinTrackers) {
		t.Fatalf("persisted metadata lost kept or builtin trackers: %v", saved.Trackers)
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
	e.SetExtraTrackers([]string{shared})
	e.SetExtraTrackers(nil)
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
	if !hasAll(announce, builtinTrackers) {
		t.Fatalf("clearing settings lost the built-in trackers: %v", announce)
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
	e.SetExtraTrackers(nil)
	e.mu.Lock()
	j := e.tasks[task.ID]
	if j == nil || j.torrent == nil {
		e.mu.Unlock()
		t.Fatal("admitted task has no torrent")
	}
	announce := j.torrent.Metainfo().AnnounceList
	e.mu.Unlock()
	if hasAll(announce, []string{extra}) {
		t.Fatalf("clearing admission extras left the tracker announced: %v", announce)
	}
	if !hasAll(announce, builtinTrackers) {
		t.Fatalf("clearing admission extras lost the built-in trackers: %v", announce)
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
	e.SetExtraTrackers(nil)
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
	if hasAll(announce, []string{extra}) {
		t.Fatalf("resumed task still announces the cleared tracker: %v", announce)
	}
	if !hasAll(announce, builtinTrackers) {
		t.Fatalf("resumed task lost the built-in trackers: %v", announce)
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
	if hasAll(announce, []string{extra}) {
		t.Fatalf("restarted task still announces the cleared tracker: %v", announce)
	}
	if !hasAll(announce, builtinTrackers) {
		t.Fatalf("restarted task lost the built-in trackers: %v", announce)
	}
}

func TestReconciledTiersOrdersUserRemoteAndBuiltins(t *testing.T) {
	own := "udp://own.example.com:1/announce"
	user := "udp://user.example.com:1/announce"
	remote := "udp://remote.example.com:1/announce"
	tiers, injectedUser, injectedRemote := reconciledTiers(
		[][]string{{own}},
		nil,
		[]string{user},
		nil,
		[]string{remote},
	)
	if len(tiers) != 4 ||
		len(tiers[0]) != 1 || tiers[0][0] != own ||
		len(tiers[1]) != 1 || tiers[1][0] != user ||
		len(tiers[2]) != 1 || tiers[2][0] != remote {
		t.Fatalf("tiers must order own, user, remote then built-ins: %v", tiers)
	}
	if !hasAll(tiers, builtinTrackers) {
		t.Fatalf("missing built-in trackers: %v", tiers)
	}
	if len(injectedUser) != 1 || injectedUser[0] != user || len(injectedRemote) != 1 || injectedRemote[0] != remote {
		t.Fatalf("injectedUser = %v, injectedRemote = %v", injectedUser, injectedRemote)
	}
	shared := "udp://shared.example.com:1/announce"
	tiers, _, injectedRemote = reconciledTiers(nil, nil, []string{shared}, nil, []string{shared})
	if len(tiers) != 2 || len(tiers[0]) != 1 || tiers[0][0] != shared || len(injectedRemote) != 0 {
		t.Fatalf("a URL both tiers configure belongs to the user tier only: %v %v", tiers, injectedRemote)
	}
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
	if !hasAll(announce, remote) || !hasAll(announce, builtinTrackers) {
		t.Fatalf("engine torrent lacks remote or built-in trackers: %v", announce)
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
	if hasAll(announce, []string{remote}) {
		t.Fatalf("clearing the remote tier left the tracker announced: %v", announce)
	}
	if !hasAll(announce, builtinTrackers) {
		t.Fatalf("clearing the remote tier lost the built-in trackers: %v", announce)
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
	if hasAll(announce, []string{remote}) {
		t.Fatalf("restarted task still announces the cleared remote tracker: %v", announce)
	}
	if !hasAll(announce, builtinTrackers) {
		t.Fatalf("restarted task lost the built-in trackers: %v", announce)
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
	e.SetExtraTrackers(nil)
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
	if !hasAll(announce, builtinTrackers) {
		t.Fatalf("clearing settings lost the built-in trackers: %v", announce)
	}
}
