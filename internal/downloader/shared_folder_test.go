package downloader

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dipper.local/dipper/internal/core"
	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
)

func layoutFixture(t *testing.T, name string, files map[string][]byte) *fixture {
	t.Helper()
	dir := canonicalTempDir(t)
	single := len(files) == 1 && files[name] != nil
	for path, data := range files {
		full := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	info := metainfo.Info{PieceLength: 16 << 10}
	target := filepath.Join(dir, name)
	if !single {
		if err := os.MkdirAll(target, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := info.BuildFromFilePath(target); err != nil {
		t.Fatal(err)
	}
	encoded, err := bencode.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	seed := &fixture{mi: metainfo.MetaInfo{InfoBytes: encoded}, files: files}
	cfg := torrent.NewDefaultClientConfig()
	localConfig(cfg)
	cfg.Seed = true
	var content []byte
	for _, file := range info.UpvertedFiles() {
		path := filepath.ToSlash(filepath.Join(append([]string{info.BestName()}, file.BestPath()...)...))
		content = append(content, files[path]...)
	}
	cfg.DefaultStorage = memorySeed{content}
	seed.seed, err = torrent.NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { seed.seed.Close() })
	seed.torrent, _, err = seed.seed.AddTorrentSpec(torrent.TorrentSpecFromMetaInfo(&seed.mi))
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "layout seed verification", 30*time.Second, func() bool { return seed.torrent.Complete().Bool() })
	seed.magnet = "magnet:?xt=urn:btih:" + seed.mi.HashInfoBytes().HexString() + "&x.pe=" + url.QueryEscape(seed.seed.ListenAddrs()[0].String())
	return seed
}

func layoutTask(t *testing.T, seed *fixture, folder string) Task {
	t.Helper()
	var encoded bytes.Buffer
	if err := seed.mi.Write(&encoded); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(encoded.Bytes())
	}))
	t.Cleanup(server.Close)
	return Task{ID: seed.mi.HashInfoBytes().HexString(), Magnet: seed.magnet, TorrentURL: server.URL, Folder: folder, Shared: true}
}

func TestSharedFolderCoreResetAndRestart(t *testing.T) {
	pack := layoutFixture(t, "release", map[string][]byte{
		"release/video.mkv":      bytes.Repeat([]byte{31}, 32<<10),
		"release/subs/字幕.ass":    []byte("subtitle"),
		"release/subs/empty.txt": {},
	})
	episode := layoutFixture(t, "[Fansub] 02.mkv", map[string][]byte{"[Fansub] 02.mkv": bytes.Repeat([]byte{42}, 32<<10)})
	root := canonicalTempDir(t)
	config := Config{DataDir: filepath.Join(root, "state")}
	engine := testEngine(t, config)
	openCore := func() *core.Core {
		t.Helper()
		state, err := core.Open(filepath.Join(root, "core.sqlite"), core.Options{
			DownloadRoot: filepath.Join(root, "downloads"), Downloader: coreAdapter{engine}, DisableScheduler: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = state.Close() })
		return state
	}
	state := openCore()
	task1 := layoutTask(t, pack, "")
	task2 := layoutTask(t, episode, "")
	resources := []core.Resource{
		{ID: "pack", Title: "Season pack", Magnet: task1.Magnet, TorrentURL: task1.TorrentURL},
		{ID: "episode", Title: "Episode 02", Magnet: task2.Magnet, TorrentURL: task2.TorrentURL},
	}
	sub, err := state.CreateSubscription(context.Background(), core.Anime{ID: "anime", Title: "Anime"}, core.Group{ID: "group", Name: "Fansub"}, core.Rule{}, resources, []string{"pack", "episode"})
	if err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(root, "downloads", "Anime", "Fansub")
	complete := func() bool {
		tasks := engine.Snapshot()
		if len(tasks) != 2 {
			return false
		}
		for _, task := range tasks {
			if task.Error != "" {
				t.Fatalf("shared task failed: %+v", task)
			}
			if task.Folder != folder {
				t.Fatalf("unexpected visible task folder: %s", task.Folder)
			}
			if !task.Complete {
				return false
			}
		}
		return true
	}
	waitFor(t, "two shared tasks completed", 30*time.Second, complete)
	checkFiles(t, pack, folder)
	checkFiles(t, episode, folder)
	for id, count := range map[string]int{task1.ID: 3, task2.ID: 1} {
		files, err := engine.DownloadFiles(id)
		if err != nil || len(files) != count {
			t.Fatalf("owned file listing for %s: %v, %v", id, files, err)
		}
	}
	if err := engine.Stop(task1.ID); err != nil {
		t.Fatal(err)
	}
	if err := state.Close(); err != nil {
		t.Fatal(err)
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	engine = testEngine(t, config)
	state = openCore()
	files, err := engine.PlayableFiles(task1.ID)
	if err != nil || len(files) != 3 {
		t.Fatalf("stopped task lost playback after restart: %v, %v", files, err)
	}
	if err := state.RemoveSubscription(context.Background(), sub.ID); err != nil {
		t.Fatal(err)
	}
	if err := state.RemoveTask(context.Background(), task1.ID, true, func() (bool, error) {
		err := engine.Remove(task1.ID)
		return err == nil || errors.Is(err, ErrRemoveCleanup), err
	}); err != nil {
		t.Fatal(err)
	}
	checkFiles(t, episode, folder)
	if _, err := os.Stat(filepath.Join(folder, "release", "video.mkv")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("reset did not delete owned pack: %v", err)
	}
	if err := state.RetryPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "independent shared reset completed", 30*time.Second, complete)
	checkFiles(t, pack, folder)
	note := filepath.Join(folder, "keep.txt")
	if err := os.WriteFile(note, []byte("unowned"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := engine.Remove(task2.ID); err != nil {
		t.Fatal(err)
	}
	checkFiles(t, pack, folder)
	if err := engine.Remove(task1.ID); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(note); err != nil || string(body) != "unowned" {
		t.Fatalf("deletion touched unowned sibling: %q, %v", body, err)
	}
}

func TestSharedFolderCollisionsAndLateMetadata(t *testing.T) {
	first := layoutFixture(t, "episode.mkv", map[string][]byte{"episode.mkv": []byte("first payload")})
	second := layoutFixture(t, "episode.mkv", map[string][]byte{"episode.mkv": []byte("other payload")})
	root := canonicalTempDir(t)
	config := Config{DataDir: filepath.Join(root, "state")}
	engine := testEngine(t, config)
	task := layoutTask(t, first, filepath.Join(root, "Fansub"))
	if err := engine.Ensure(task); err != nil {
		t.Fatalf("shared alias admission: %v", err)
	}
	waitFor(t, "first episode complete", 30*time.Second, func() bool { return one(t, engine).Complete })
	if err := engine.Pause(task.ID); err != nil {
		t.Fatal(err)
	}
	if err := engine.Ensure(task); err != nil || len(engine.Snapshot()) != 1 {
		t.Fatalf("same hash replay was not idempotent: %v", err)
	}
	collision := layoutTask(t, second, task.Folder)
	if err := engine.Ensure(collision); err == nil {
		t.Fatal("second hash was allowed to overwrite a paused owner's file")
	}
	checkFiles(t, first, task.Folder)
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	engine = testEngine(t, config)
	late := collision
	late.TorrentURL = ""
	if err := engine.Ensure(late); err != nil {
		t.Fatalf("magnet identity should admit before metadata: %v", err)
	}
	waitFor(t, "late metadata collision observed", 60*time.Second, func() bool {
		for _, snapshot := range engine.Snapshot() {
			if snapshot.ID == late.ID {
				return snapshot.Error != "" || snapshot.TotalBytes > 0
			}
		}
		return false
	})
	for _, snapshot := range engine.Snapshot() {
		if snapshot.ID == late.ID && snapshot.Error == "" {
			t.Fatalf("late metadata bypassed stopped-owner claim: %+v", snapshot)
		}
	}
	checkFiles(t, first, task.Folder)
	if err := engine.Remove(late.ID); err != nil {
		t.Fatal(err)
	}
	checkFiles(t, first, task.Folder)
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	engine = testEngine(t, config)
	if files, err := engine.PlayableFiles(task.ID); err != nil || len(files) != 1 {
		t.Fatalf("paused owner lost after failed late metadata restart: %v, %v", files, err)
	}
	checkFiles(t, first, task.Folder)
}

func TestSharedFolderClaimsOutlivePolicyChanges(t *testing.T) {
	seed := layoutFixture(t, "episode.mkv", map[string][]byte{"episode.mkv": []byte("owned")})
	other := layoutFixture(t, "episode.mkv", map[string][]byte{"episode.mkv": []byte("other")})
	root := canonicalTempDir(t)
	engine := testEngine(t, Config{DataDir: filepath.Join(root, "state")})
	folder := filepath.Join(root, "Fansub")
	task := layoutTask(t, seed, folder)
	if err := engine.Ensure(task); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "policy owner complete", 30*time.Second, func() bool { return one(t, engine).Complete })
	collision := layoutTask(t, other, folder)
	states := []struct {
		name   string
		change func(bool) error
	}{
		{"paused", func(active bool) error {
			if active {
				return engine.Pause(task.ID)
			}
			return engine.Resume(task.ID)
		}},
		{"stopped", func(active bool) error {
			if active {
				return engine.Stop(task.ID)
			}
			return engine.Resume(task.ID)
		}},
		{"auto-stopped", func(active bool) error {
			engine.mu.Lock()
			defer engine.mu.Unlock()
			j := engine.tasks[task.ID]
			old := j.Policy.AutoStopped
			j.Policy.AutoStopped = active
			if err := engine.saveLocked(); err != nil {
				j.Policy.AutoStopped = old
				return err
			}
			return nil
		}},
	}
	for _, state := range states {
		t.Run(state.name, func(t *testing.T) {
			if err := state.change(true); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = state.change(false) })
			if err := engine.Ensure(task); err != nil {
				t.Fatalf("same-hash replay rejected while %s: %v", state.name, err)
			}
			if err := engine.Ensure(collision); err == nil {
				t.Fatalf("%s owner's file was claimable", state.name)
			}
			checkFiles(t, seed, folder)
		})
	}
}

func TestSharedFolderOwnershipConcurrency(t *testing.T) {
	owners, err := openOwnership(canonicalTempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	folder := canonicalTempDir(t)
	const tasks = 12
	start := make(chan struct{})
	impls := make(chan storage.TorrentImpl, tasks)
	var wg sync.WaitGroup
	for index := range tasks {
		wg.Go(func() {
			<-start
			var failure atomic.Pointer[taskFailure]
			store := diskStorage{folder: folder, ownership: owners, task: Task{ID: fmt.Sprintf("%02d", index), Folder: folder, Shared: true}, failure: &failure, completion: storage.NewMapPieceCompletion()}
			name := fmt.Sprintf("episode-%02d.mkv", index)
			impl, err := store.OpenTorrent(context.Background(), &metainfo.Info{Name: name, Length: 1, PieceLength: 16384}, metainfo.Hash{byte(index)})
			if err != nil {
				t.Errorf("concurrent admission %d: %v", index, err)
				return
			}
			impls <- impl
		})
	}
	close(start)
	wg.Wait()
	close(impls)
	for impl := range impls {
		if err := impl.Close(); err != nil {
			t.Error(err)
		}
	}
	if got := len(owners.claims); got != tasks {
		t.Fatalf("concurrent claims: got %d, want %d", got, tasks)
	}
	var removeWG sync.WaitGroup
	for index := range tasks {
		removeWG.Go(func() {
			task := Task{ID: fmt.Sprintf("%02d", index), Folder: folder, Shared: true}
			if err := owners.remove(task); err != nil {
				t.Errorf("concurrent cleanup %d: %v", index, err)
			}
			if err := owners.release(task.ID); err != nil {
				t.Errorf("concurrent release %d: %v", index, err)
			}
		})
	}
	removeWG.Wait()
	if len(owners.claims) != 0 {
		t.Fatalf("cleanup left claims: %+v", owners.claims)
	}
	entries, err := os.ReadDir(folder)
	if err != nil || len(entries) != 0 {
		t.Fatalf("cleanup left payload: %v, %v", entries, err)
	}
}

func TestSharedFolderConcurrentSamePathOneOwner(t *testing.T) {
	owners, err := openOwnership(canonicalTempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	folder := canonicalTempDir(t)
	start := make(chan struct{})
	results := make(chan error, 2)
	for index := range 2 {
		go func() {
			<-start
			var failure atomic.Pointer[taskFailure]
			store := diskStorage{folder: folder, ownership: owners, task: Task{ID: fmt.Sprintf("same-%d", index), Folder: folder, Shared: true}, failure: &failure, completion: storage.NewMapPieceCompletion()}
			impl, err := store.OpenTorrent(context.Background(), &metainfo.Info{Name: "episode.mkv", Length: 1, PieceLength: 16384}, metainfo.Hash{byte(index)})
			if err == nil {
				err = impl.Close()
			}
			results <- err
		}()
	}
	close(start)
	first, second := <-results, <-results
	if (first == nil) == (second == nil) {
		t.Fatalf("same-path admission results: %v, %v", first, second)
	}
	if len(owners.claims) != 1 {
		t.Fatalf("same-path claims: %+v", owners.claims)
	}
}

func TestSharedFolderManifestSafety(t *testing.T) {
	cases := []struct {
		name  string
		files []metainfo.FileInfo
	}{
		{"case", []metainfo.FileInfo{{Path: []string{"Subs", "a.ass"}}, {Path: []string{"subs", "b.ass"}}}},
		{"prefix", []metainfo.FileInfo{{Path: []string{"subs"}}, {Path: []string{"subs", "b.ass"}}}},
		{"escape", []metainfo.FileInfo{{Path: []string{"ok.mkv"}}, {Path: []string{"..", "b.ass"}}}},
	}
	for _, testcase := range cases {
		t.Run(testcase.name, func(t *testing.T) {
			folder := canonicalTempDir(t)
			owners, err := openOwnership(canonicalTempDir(t))
			if err != nil {
				t.Fatal(err)
			}
			var failure atomic.Pointer[taskFailure]
			store := diskStorage{folder: folder, ownership: owners, task: Task{ID: "one", Folder: folder, Shared: true}, failure: &failure, completion: storage.NewMapPieceCompletion()}
			info := &metainfo.Info{Name: "release", PieceLength: 16384, Files: testcase.files}
			if _, err := store.OpenTorrent(context.Background(), info, metainfo.Hash{}); err == nil {
				t.Fatal("unsafe manifest admitted")
			}
			entries, err := os.ReadDir(folder)
			if err != nil || len(entries) != 0 {
				t.Fatalf("invalid manifest mutated payload before validation: %v, %v", entries, err)
			}
		})
	}
	t.Run("unowned", func(t *testing.T) {
		folder := canonicalTempDir(t)
		existing := filepath.Join(folder, "episode.mkv")
		if err := os.WriteFile(existing, []byte("mine"), 0644); err != nil {
			t.Fatal(err)
		}
		owners, err := openOwnership(canonicalTempDir(t))
		if err != nil {
			t.Fatal(err)
		}
		var failure atomic.Pointer[taskFailure]
		store := diskStorage{folder: folder, ownership: owners, task: Task{ID: "one", Folder: folder, Shared: true}, failure: &failure, completion: storage.NewMapPieceCompletion()}
		if _, err := store.OpenTorrent(context.Background(), &metainfo.Info{Name: "episode.mkv", Length: 10, PieceLength: 16384}, metainfo.Hash{}); err == nil {
			t.Fatal("unowned regular file adopted")
		}
		if err := owners.remove(store.task); err != nil {
			t.Fatal(err)
		}
		if body, err := os.ReadFile(existing); err != nil || string(body) != "mine" {
			t.Fatalf("unowned file changed: %q, %v", body, err)
		}
	})
}

func TestSharedFolderRemovalJournalAndHardlink(t *testing.T) {
	seed := layoutFixture(t, "episode.mkv", map[string][]byte{"episode.mkv": []byte("payload")})
	root := canonicalTempDir(t)
	config := Config{DataDir: filepath.Join(root, "state")}
	engine := testEngine(t, config)
	task := layoutTask(t, seed, filepath.Join(root, "Fansub"))
	if err := engine.Ensure(task); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "removal fixture complete", 30*time.Second, func() bool { return one(t, engine).Complete })
	if err := engine.Stop(task.ID); err != nil {
		t.Fatal(err)
	}
	protected := filepath.Join(root, "protected.mkv")
	if err := os.WriteFile(protected, []byte("do not touch"), 0644); err != nil {
		t.Fatal(err)
	}
	owned := filepath.Join(task.Folder, "episode.mkv")
	if err := os.Remove(owned); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(protected, owned); err != nil {
		t.Fatal(err)
	}
	if files, err := engine.PlayableFiles(task.ID); err != nil || len(files) != 0 {
		t.Fatalf("hardlink offered for playback: %v, %v", files, err)
	}
	if err := engine.Remove(task.ID); !errors.Is(err, ErrRemoveCleanup) {
		t.Fatalf("unsafe removal must retain cleanup journal: %v", err)
	}
	if tasks := engine.Snapshot(); len(tasks) != 0 {
		t.Fatalf("removed task resurrected: %v", tasks)
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	engine = testEngine(t, config)
	if err := engine.Ensure(task); err == nil {
		t.Fatal("readmission ignored failed cleanup and its hardlink")
	}
	if _, err := engine.ownership.files(task); err != nil {
		t.Fatalf("failed cleanup released ownership: %v", err)
	}
	if err := os.Remove(owned); err != nil {
		t.Fatal(err)
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	engine = testEngine(t, config)
	body, err := os.ReadFile(filepath.Join(config.DataDir, "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved checkpoint
	if err := json.Unmarshal(body, &saved); err != nil || len(saved.Removing) != 0 || len(saved.Tasks) != 0 {
		t.Fatalf("cleanup did not recover on restart: %s, %v", body, err)
	}
	if body, err := os.ReadFile(protected); err != nil || string(body) != "do not touch" {
		t.Fatalf("cleanup changed external payload: %q, %v", body, err)
	}
	if err := engine.Ensure(task); err != nil {
		t.Fatalf("recovered removal cannot be admitted: %v", err)
	}
}

func TestLegacyCheckpointFolderReplay(t *testing.T) {
	seed := layoutFixture(t, "episode.mkv", map[string][]byte{"episode.mkv": []byte("legacy payload")})
	root := canonicalTempDir(t)
	config := Config{DataDir: filepath.Join(root, "state")}
	engine := testEngine(t, config)
	task := layoutTask(t, seed, filepath.Join(root, "Anime", strings.Repeat("a", 40)))
	task.Shared = false
	if err := engine.Ensure(task); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "legacy fixture complete", 30*time.Second, func() bool { return one(t, engine).Complete })
	if err := engine.Stop(task.ID); err != nil {
		t.Fatal(err)
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(config.DataDir, "tasks.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved checkpoint
	if err := json.Unmarshal(body, &saved); err != nil {
		t.Fatal(err)
	}
	saved.Version = 1
	body, err = json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(config.DataDir, "files.json")); err != nil {
		t.Fatal(err)
	}
	task.TorrentURL = ""
	engine = testEngine(t, config)
	if err := engine.Ensure(task); err != nil {
		t.Fatal(err)
	}
	snapshot := one(t, engine)
	if snapshot.Folder != task.Folder || !snapshot.ManuallyStopped {
		t.Fatalf("legacy destination or intent moved: %+v", snapshot)
	}
	files, err := engine.PlayableFiles(task.ID)
	if err != nil || len(files) != 1 || files[0] != filepath.Join(task.Folder, "episode.mkv") {
		t.Fatalf("legacy playback unavailable: %v, %v", files, err)
	}
	checkFiles(t, seed, task.Folder)
	if err := engine.Resume(task.ID); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "legacy local resume", 30*time.Second, func() bool { return one(t, engine).Complete })
	if err := engine.Remove(task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(task.Folder); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("empty legacy destination not removed: %v", err)
	}
}

func TestSharedFolderAliasAndInnerReparse(t *testing.T) {
	root := canonicalTempDir(t)
	actual := filepath.Join(root, "actual")
	if err := os.Mkdir(actual, 0755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if runtime.GOOS == "windows" {
		if body, err := exec.Command("cmd", "/c", "mklink", "/J", alias, actual).CombinedOutput(); err != nil {
			t.Fatalf("create junction: %s, %v", body, err)
		}
	} else if err := os.Symlink(actual, alias); err != nil {
		t.Fatal(err)
	}
	engine := testEngine(t, Config{DataDir: filepath.Join(root, "state")})
	hash := strings.Repeat("ab", 20)
	task := Task{ID: hash, Magnet: "magnet:?xt=urn:btih:" + hash, Folder: filepath.Join(alias, "Fansub"), Shared: true}
	if err := engine.Ensure(task); err != nil {
		t.Fatal(err)
	}
	if err := engine.Ensure(task); err != nil {
		t.Fatalf("shared selected-root alias replay rejected: %v", err)
	}
	if len(engine.Snapshot()) != 1 {
		t.Fatal("alias replay duplicated task")
	}
	inside := filepath.Join(actual, "Fansub")
	link := filepath.Join(inside, "release")
	if err := os.Symlink(actual, link); err != nil {
		if runtime.GOOS != "windows" {
			t.Fatal(err)
		}
		if body, err := exec.Command("cmd", "/c", "mklink", "/J", link, actual).CombinedOutput(); err != nil {
			t.Fatalf("create inner junction: %s, %v", body, err)
		}
	}
	var failure atomic.Pointer[taskFailure]
	store := diskStorage{folder: inside, task: Task{ID: "reparse", Folder: inside, Shared: true}, ownership: engine.ownership, failure: &failure, completion: storage.NewMapPieceCompletion()}
	info := &metainfo.Info{Name: "release", Files: []metainfo.FileInfo{{Path: []string{"escape.mkv"}, Length: 1}}, PieceLength: 16384}
	if _, err := store.OpenTorrent(context.Background(), info, metainfo.Hash{}); err == nil {
		t.Fatal("shared storage followed inner reparse point")
	}
	if _, err := os.Stat(filepath.Join(actual, "escape.mkv")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("inner reparse created external file: %v", err)
	}
}
