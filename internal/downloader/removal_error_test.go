package downloader

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
