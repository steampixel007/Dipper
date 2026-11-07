package downloader

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNonSharedRemovalAfterDeletedPayloadLeavesNoJournal(t *testing.T) {
	seed := layoutFixture(t, "season", map[string][]byte{
		"season/a.mkv":      bytes.Repeat([]byte{7}, 32<<10),
		"season/Subs/b.srt": []byte("subtitle"),
	})
	root := canonicalTempDir(t)
	config := Config{DataDir: filepath.Join(root, "state")}
	e := testEngine(t, config)
	task := layoutTask(t, seed, filepath.Join(root, "content"))
	task.Shared = false
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "non-shared fixture complete", 30*time.Second, func() bool { return one(t, e).Complete })
	if err := e.Stop(task.ID); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(task.Folder, "season", "Subs", "b.srt")); err != nil {
		t.Fatal(err)
	}
	if err := e.Remove(task.ID); err != nil {
		t.Fatalf("removal after a deleted payload file = %v", err)
	}
	if _, err := os.Stat(task.Folder); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removal left the destination behind: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(config.DataDir, "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved checkpoint
	if err := json.Unmarshal(body, &saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.Removing) != 0 {
		t.Fatalf("successful removal kept its journal: %+v", saved.Removing)
	}
	if err := e.Ensure(task); err != nil {
		t.Fatalf("readmission after removal: %v", err)
	}
}

func TestRemoveCleanupErrorKeepsTaskForgotten(t *testing.T) {
	root := t.TempDir()
	config := Config{DataDir: filepath.Join(root, "state")}
	e := testEngine(t, config)
	task := Task{ID: strings.Repeat("ab", 20), Magnet: "magnet:?xt=urn:btih:" + strings.Repeat("ab", 20), Folder: filepath.Join(root, "content")}
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	if err := e.Stop(task.ID); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	owned := e.tasks[task.ID].Task
	e.mu.Unlock()
	e.ownership.mu.Lock()
	err := e.ownership.reserve(owned, strings.Repeat("ab", 20), nil)
	e.ownership.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	metadata := e.metadataPath(task.ID)
	if err := os.MkdirAll(metadata, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metadata, "held"), []byte("cannot remove a nonempty directory as metadata"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := e.Remove(task.ID); !errors.Is(err, ErrRemoveCleanup) {
		t.Fatalf("committed removal cleanup error = %v; caller needs to forget its delivery record", err)
	}
	if tasks := e.Snapshot(); len(tasks) != 0 {
		t.Fatalf("cleanup failure restored task: %+v", tasks)
	}
	body, err := os.ReadFile(filepath.Join(config.DataDir, "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved checkpoint
	if err := json.Unmarshal(body, &saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.Tasks) != 0 {
		t.Fatalf("cleanup failure restored durable task: %+v", saved.Tasks)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := testEngine(t, config)
	if tasks := reopened.Snapshot(); len(tasks) != 0 {
		t.Fatalf("restart resurrected removed task: %+v", tasks)
	}
}

func TestRemoveCheckpointFailurePreservesTask(t *testing.T) {
	root := t.TempDir()
	e := testEngine(t, Config{DataDir: filepath.Join(root, "state")})
	task := Task{ID: strings.Repeat("cd", 20), Magnet: "magnet:?xt=urn:btih:" + strings.Repeat("cd", 20), Folder: filepath.Join(root, "content")}
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	if err := e.Stop(task.ID); err != nil {
		t.Fatal(err)
	}
	checkpointPath := filepath.Join(e.dir, "tasks.json")
	if err := os.Remove(checkpointPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(checkpointPath, 0700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(checkpointPath); err != nil {
			t.Error(err)
		}
	})
	if err := e.Remove(task.ID); err == nil || errors.Is(err, ErrRemoveCleanup) {
		t.Fatalf("uncommitted checkpoint failure = %v; must preserve delivery intent", err)
	}
	if tasks := e.Snapshot(); len(tasks) != 1 || tasks[0].ID != task.ID {
		t.Fatalf("uncommitted removal lost task: %+v", tasks)
	}
}

func TestLegacyFolderCleanupErrorKeepsRemovalJournal(t *testing.T) {
	root := t.TempDir()
	config := Config{DataDir: filepath.Join(root, "state")}
	e := testEngine(t, config)
	task := Task{ID: strings.Repeat("ef", 20), Magnet: "magnet:?xt=urn:btih:" + strings.Repeat("ef", 20), Folder: filepath.Join(root, "content")}
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	if err := e.Stop(task.ID); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(task.Folder, "keep.txt")
	if err := os.WriteFile(keep, []byte("not owned by the task"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := e.Remove(task.ID); !errors.Is(err, ErrRemoveCleanup) {
		t.Fatalf("legacy folder cleanup error = %v; unowned files must not be hidden", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("unowned file was removed: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(config.DataDir, "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved checkpoint
	if err := json.Unmarshal(body, &saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.Tasks) != 0 || len(saved.Removing) != 1 {
		t.Fatalf("cleanup failure was not journaled: %+v", saved)
	}
}

func TestLegacyUnreadableMetadataDoesNotBlockStartup(t *testing.T) {
	root := t.TempDir()
	config := Config{DataDir: filepath.Join(root, "state")}
	e := testEngine(t, config)
	task := Task{ID: strings.Repeat("12", 20), Magnet: "magnet:?xt=urn:btih:" + strings.Repeat("12", 20), Folder: filepath.Join(root, "content")}
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	if err := e.Stop(task.ID); err != nil {
		t.Fatal(err)
	}
	metadata := e.metadataPath(task.ID)
	if err := os.MkdirAll(metadata, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metadata, "held"), []byte("unreadable metadata"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := testEngine(t, config)
	tasks := reopened.Snapshot()
	if len(tasks) != 1 || tasks[0].ID != task.ID {
		t.Fatalf("legacy task was lost on unreadable metadata: %+v", tasks)
	}
}

func TestLegacyUnreadableMetadataAllowsEmptyFolderRemoval(t *testing.T) {
	root := t.TempDir()
	e := testEngine(t, Config{DataDir: filepath.Join(root, "state")})
	task := Task{ID: strings.Repeat("34", 20), Magnet: "magnet:?xt=urn:btih:" + strings.Repeat("34", 20), Folder: filepath.Join(root, "content")}
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	if err := e.Stop(task.ID); err != nil {
		t.Fatal(err)
	}
	metadata := e.metadataPath(task.ID)
	if err := os.MkdirAll(metadata, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := e.Remove(task.ID); err != nil {
		t.Fatalf("empty legacy folder removal = %v", err)
	}
	if tasks := e.Snapshot(); len(tasks) != 0 {
		t.Fatalf("removed task survived: %+v", tasks)
	}
	if _, err := os.Stat(task.Folder); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("empty legacy folder survived removal: %v", err)
	}
}

func TestLegacyInvalidMetadataStillBlocksStartup(t *testing.T) {
	root := t.TempDir()
	config := Config{DataDir: filepath.Join(root, "state")}
	e := testEngine(t, config)
	task := Task{ID: strings.Repeat("56", 20), Magnet: "magnet:?xt=urn:btih:" + strings.Repeat("56", 20), Folder: filepath.Join(root, "content")}
	if err := e.Ensure(task); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.metadataPath(task.ID), []byte("not torrent metadata"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	if reopened, err := newEngine(config, localConfig); err == nil {
		_ = reopened.Close()
		t.Fatal("structurally invalid legacy metadata was ignored")
	}
}
