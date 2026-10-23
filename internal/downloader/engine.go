// Package downloader embeds the BitTorrent client and owns durable transfer intent.
package downloader

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
	"golang.org/x/time/rate"
)

type Config struct {
	DataDir     string
	UploadLimit int64 // Bytes per second; zero selects the default 1 MiB/s, negative means unlimited.
}

type Task struct {
	ID         string          `json:"id"`
	Magnet     string          `json:"magnet,omitempty"`
	TorrentURL string          `json:"torrent_url,omitempty"`
	Folder     string          `json:"folder"`
	Context    context.Context `json:"-"`
}

type Snapshot struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	InfoHash        string  `json:"info_hash"`
	Folder          string  `json:"folder"`
	Status          string  `json:"status"`
	Error           string  `json:"error,omitempty"`
	TotalBytes      int64   `json:"total_bytes"`
	CompletedBytes  int64   `json:"completed_bytes"`
	DownloadedBytes int64   `json:"downloaded_bytes"`
	UploadedBytes   int64   `json:"uploaded_bytes"`
	DownloadSpeed   int64   `json:"download_speed"`
	UploadSpeed     int64   `json:"upload_speed"`
	SeedingSeconds  int64   `json:"seeding_seconds"`
	Progress        float64 `json:"progress"`
	Complete        bool    `json:"complete"`
	Paused          bool    `json:"paused"`
	ManuallyStopped bool    `json:"manually_stopped"`
	AutoStopped     bool    `json:"auto_stopped"`
}

const (
	StatusMetadata    = "metadata"
	StatusDownloading = "downloading"
	StatusSeeding     = "seeding"
	StatusPaused      = "paused"
	StatusStopped     = "stopped"
	StatusCompleted   = "completed"
	StatusError       = "error"
)

var (
	ErrClosed      = errors.New("download engine is closed")
	ErrNotFound    = errors.New("download task not found")
	ErrNotComplete = errors.New("download is not complete")
)

type savedTask struct {
	Task
	Policy         policyState `json:"policy"`
	Name           string      `json:"name"`
	InfoHash       string      `json:"info_hash"`
	TotalBytes     int64       `json:"total_bytes"`
	CompletedBytes int64       `json:"completed_bytes"`
	Complete       bool        `json:"complete"`
}

type checkpoint struct {
	Version int         `json:"version"`
	Tasks   []savedTask `json:"tasks"`
}

type entry struct {
	savedTask
	torrent       *torrent.Torrent
	failure       atomic.Pointer[taskFailure]
	generation    atomic.Uint64
	previous      observation
	metadataSaved bool
	requested     bool
	downSpeed     int64
	upSpeed       int64
	err           error
}

type Engine struct {
	mu            sync.Mutex
	client        *torrent.Client
	completion    storage.PieceCompletion
	limiter       *rate.Limiter
	dir           string
	tasks         map[string]*entry
	ctx           context.Context
	cancel        context.CancelFunc
	done          chan struct{}
	closed        bool
	closeErr      error
	suspended     bool
	network       *bool
	now           func() time.Time
	awake         func() time.Duration
	online        func() bool
	checkpointErr error
}

func New(config Config) (*Engine, error) {
	return newEngine(config, nil)
}

func newEngine(config Config, configure func(*torrent.ClientConfig)) (_ *Engine, err error) {
	if strings.TrimSpace(config.DataDir) == "" {
		return nil, errors.New("download state directory is required")
	}
	dir, err := filepath.Abs(config.DataDir)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if dir, err = filepath.EvalSymlinks(dir); err != nil {
		return nil, err
	}
	e := &Engine{dir: dir, tasks: make(map[string]*entry), done: make(chan struct{}), now: time.Now, awake: awakeTime, online: networkAvailable}
	e.ctx, e.cancel = context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			e.cancel()
			if e.client != nil {
				e.client.Close()
			}
			if e.completion != nil {
				e.completion.Close()
			}
		}
	}()
	e.completion, err = storage.NewDefaultPieceCompletionForDir(filepath.Join(dir, "pieces"))
	if err != nil {
		return nil, fmt.Errorf("open piece checkpoint: %w", err)
	}
	if err = e.load(); err != nil {
		return nil, err
	}
	limit := config.UploadLimit
	if limit == 0 {
		limit = 1 << 20
	}
	e.limiter = rate.NewLimiter(uploadRate(limit), 32<<10)
	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = dir
	cfg.DefaultStorage = noStorage{}
	cfg.ListenPort = 0
	cfg.Seed = true
	cfg.UploadRateLimiter = e.limiter
	if configure != nil {
		configure(cfg)
	}
	e.client, err = torrent.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("start BitTorrent client: %w", err)
	}
	for _, task := range e.tasks {
		if task.Policy.Paused || task.Policy.ManuallyStopped || task.Policy.AutoStopped {
			continue
		}
		if startErr := e.startLocked(task, e.ctx); startErr != nil {
			task.err = startErr
		}
	}
	go e.run()
	return e, nil
}

// Ensure is idempotent. Existing tasks retain their user intent; Resume explicitly changes it.
// Context cancels admission/metainfo fetching, not the lifetime of an admitted download.
func (e *Engine) Ensure(task Task) error {
	ctx := task.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(task.ID) == "" || strings.TrimSpace(task.Folder) == "" {
		return errors.New("task ID and isolated folder are required")
	}
	if task.Magnet != "" {
		spec, err := sourceSpec(ctx, Task{Magnet: task.Magnet})
		if err != nil {
			return err
		}
		if err := validateIdentity(task.ID, spec); err != nil {
			return err
		}
	}
	folder, err := filepath.Abs(task.Folder)
	if err != nil {
		return err
	}
	if canonical, canonicalErr := filepath.EvalSymlinks(folder); canonicalErr == nil {
		folder = canonical
	} else if !errors.Is(canonicalErr, os.ErrNotExist) {
		return canonicalErr
	}
	task.Folder, task.Context = filepath.Clean(folder), nil
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return ErrClosed
	}
	if existing := e.tasks[task.ID]; existing != nil {
		err = sameTask(existing.Task, task)
		e.mu.Unlock()
		return err
	}
	e.mu.Unlock()

	requestCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(e.ctx, cancel)
	defer func() { stop(); cancel() }()
	spec, err := sourceSpec(requestCtx, task)
	if err != nil {
		return err
	}
	if err = requestCtx.Err(); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return ErrClosed
	}
	if err := requestCtx.Err(); err != nil {
		return err
	}
	if existing := e.tasks[task.ID]; existing != nil {
		return sameTask(existing.Task, task)
	}
	if err = os.MkdirAll(task.Folder, 0o755); err != nil {
		return err
	}
	if task.Folder, err = filepath.EvalSymlinks(task.Folder); err != nil {
		return err
	}
	for _, other := range e.tasks {
		if overlappingFolders(other.Folder, task.Folder) {
			return fmt.Errorf("folder overlaps task %q", other.ID)
		}
	}
	if overlappingFolders(e.dir, task.Folder) {
		return errors.New("content folder must be isolated from engine state")
	}
	j := &entry{savedTask: savedTask{Task: task}}
	if err = e.addLocked(j, spec); err != nil {
		return err
	}
	if err := requestCtx.Err(); err != nil {
		j.torrent.Drop()
		return err
	}
	e.tasks[j.ID] = j
	if err = e.saveLocked(); err != nil {
		delete(e.tasks, j.ID)
		j.torrent.Drop()
		return err
	}
	e.enableLocked(j)
	return nil
}

func sameTask(a, b Task) error {
	if !samePath(a.Folder, b.Folder) {
		return errors.New("task ID already belongs to a different folder")
	}
	return nil
}

func sourceSpec(ctx context.Context, task Task) (*torrent.TorrentSpec, error) {
	var magnet *torrent.TorrentSpec
	if task.Magnet != "" {
		spec, err := torrent.TorrentSpecFromMagnetUri(task.Magnet)
		if err != nil {
			return nil, fmt.Errorf("invalid magnet: %w", err)
		}
		if spec.InfoHash == (metainfo.Hash{}) {
			return nil, errors.New("magnet requires a v1 BT info hash")
		}
		magnet = spec
		if task.TorrentURL == "" {
			return spec, nil
		}
	}
	spec, err := fetchTorrentSpec(ctx, task.TorrentURL)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if magnet != nil {
			return magnet, nil
		}
		return nil, err
	}
	if magnet != nil {
		if magnet.InfoHash != spec.InfoHash {
			return nil, errors.New("torrent metadata does not match magnet info hash")
		}
		spec.PeerAddrs = append(spec.PeerAddrs, magnet.PeerAddrs...)
		spec.Trackers = append(spec.Trackers, magnet.Trackers...)
	}
	return spec, nil
}

func fetchTorrentSpec(ctx context.Context, source string) (*torrent.TorrentSpec, error) {
	u, err := url.Parse(source)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, errors.New("an HTTP(S) torrent URL or magnet is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Dipper/1")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch torrent: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("torrent source returned HTTP %d", resp.StatusCode)
	}
	const maxMetainfo = 16 << 20
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxMetainfo+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxMetainfo {
		return nil, errors.New("torrent metadata exceeds 16 MiB")
	}
	return decodeSpec(b)
}

func decodeSpec(b []byte) (*torrent.TorrentSpec, error) {
	mi, err := metainfo.Load(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("invalid torrent metadata: %w", err)
	}
	spec, err := torrent.TorrentSpecFromMetaInfoErr(mi)
	if err == nil && spec.InfoHash == (metainfo.Hash{}) {
		err = errors.New("torrent requires a v1 BT info hash")
	}
	return spec, err
}

func (e *Engine) metadataPath(id string) string {
	sum := sha256.Sum256([]byte(id))
	return filepath.Join(e.dir, hex.EncodeToString(sum[:])+".torrent")
}

func (e *Engine) startLocked(j *entry, ctx context.Context) error {
	if err := os.MkdirAll(j.Folder, 0o755); err != nil {
		return err
	}
	b, err := os.ReadFile(e.metadataPath(j.ID))
	var spec *torrent.TorrentSpec
	if err == nil {
		spec, err = decodeSpec(b)
	} else if errors.Is(err, os.ErrNotExist) {
		spec, err = sourceSpec(ctx, j.Task)
	}
	if err != nil {
		return err
	}
	if j.Magnet != "" {
		magnet, parseErr := sourceSpec(ctx, Task{Magnet: j.Magnet})
		if parseErr != nil {
			return parseErr
		}
		if len(spec.InfoBytes) > 0 && (magnet.InfoHash != (metainfo.Hash{}) && magnet.InfoHash != spec.InfoHash || magnet.InfoHashV2.Ok && magnet.InfoHashV2 != spec.InfoHashV2) {
			return errors.New("cached metadata does not match magnet")
		}
		spec.PeerAddrs = append(spec.PeerAddrs, magnet.PeerAddrs...)
		spec.Trackers = append(spec.Trackers, magnet.Trackers...)
	}
	if err = e.addLocked(j, spec); err != nil {
		return err
	}
	e.enableLocked(j)
	return nil
}

func (e *Engine) addLocked(j *entry, spec *torrent.TorrentSpec) error {
	if err := validateIdentity(j.ID, spec); err != nil {
		return err
	}
	for _, other := range e.tasks {
		if other == j {
			continue
		}
		if spec.InfoHash != (metainfo.Hash{}) && other.InfoHash == spec.InfoHash.HexString() {
			return fmt.Errorf("content already belongs to task %q", other.ID)
		}
	}
	j.failure.Store(nil)
	epoch := j.generation.Add(1)
	spec.Storage = &diskStorage{folder: j.Folder, completion: e.completion, failure: &j.failure}
	spec.DisallowDataDownload, spec.DisallowDataUpload = true, true
	spec.IgnoreUnverifiedPieceCompletion = true
	t, added, err := e.client.AddTorrentSpec(spec)
	if err != nil {
		return fmt.Errorf("add torrent: %w", err)
	}
	if !added {
		return errors.New("torrent is already attached to another task")
	}
	if failure := j.failure.Load(); failure != nil {
		t.Drop()
		return failure.err
	}
	j.torrent, j.err, j.requested, j.metadataSaved = t, nil, false, false
	j.InfoHash = t.InfoHash().HexString()
	j.Complete = false
	j.previous = observation{at: e.now(), awake: e.awake()}
	t.SetOnWriteChunkError(func(err error) {
		if j.generation.Load() == epoch {
			j.failure.Store(&taskFailure{err})
		}
	})
	if err = e.saveMetadataLocked(j); err != nil {
		t.Drop()
		j.torrent = nil
		return err
	}
	return nil
}

func validateIdentity(id string, spec *torrent.TorrentSpec) error {
	if len(id) != 40 {
		return nil
	}
	expected, err := hex.DecodeString(id)
	if err != nil {
		return errors.New("task identity must be a hexadecimal info hash")
	}
	actual := spec.InfoHash
	if actual == (metainfo.Hash{}) && spec.InfoHashV2.Ok {
		actual = *spec.InfoHashV2.Value.ToShort()
	}
	if !bytes.Equal(expected, actual[:]) {
		return fmt.Errorf("task info hash %s does not match source %s", id, actual.HexString())
	}
	return nil
}

func (e *Engine) saveMetadataLocked(j *entry) error {
	if j.torrent == nil || j.torrent.Info() == nil || j.metadataSaved {
		return nil
	}
	mi := j.torrent.Metainfo()
	var b bytes.Buffer
	if err := mi.Write(&b); err != nil {
		return err
	}
	if err := atomicWrite(e.metadataPath(j.ID), b.Bytes()); err != nil {
		return fmt.Errorf("save torrent metadata: %w", err)
	}
	j.metadataSaved = true
	return nil
}

func (e *Engine) enableLocked(j *entry) {
	if e.closed || j.torrent == nil || j.err != nil || j.Policy.Paused || j.Policy.ManuallyStopped || j.Policy.AutoStopped {
		return
	}
	j.torrent.AllowDataDownload()
	j.torrent.AllowDataUpload()
	if j.torrent.Info() != nil && !j.requested {
		j.torrent.DownloadAll()
		j.requested = true
	}
}

func (e *Engine) Pause(id string) error       { return e.changeIntent(id, "pause") }
func (e *Engine) Resume(id string) error      { return e.changeIntent(id, "resume") }
func (e *Engine) StopSeeding(id string) error { return e.changeIntent(id, "stop") }

func (e *Engine) changeIntent(id, action string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return ErrClosed
	}
	j := e.tasks[id]
	if j == nil {
		return ErrNotFound
	}
	e.sampleLocked(j, e.now(), e.awake(), e.isOnlineLocked())
	if action == "stop" && !j.Complete {
		return ErrNotComplete
	}
	old := j.Policy
	switch action {
	case "pause":
		j.Policy.Paused = true
	case "stop":
		j.Policy.ManuallyStopped = true
	case "resume":
		j.Policy.Paused, j.Policy.ManuallyStopped = false, false
	}
	if err := e.saveLocked(); err != nil {
		j.Policy = old
		return err
	}
	if action != "resume" {
		return errors.Join(e.detachLocked(j), e.saveLocked())
	}
	j.err = nil
	if j.torrent == nil && !j.Policy.AutoStopped {
		if err := e.startLocked(j, e.ctx); err != nil {
			j.err = err
			return err
		}
	}
	e.enableLocked(j)
	return nil
}

func (e *Engine) detachLocked(j *entry) error {
	if j.torrent == nil {
		return nil
	}
	metaErr := e.saveMetadataLocked(j)
	j.torrent.Drop()
	stats := j.torrent.Stats()
	j.Policy.UploadedBytes += counterDelta(j.previous.up, stats.BytesWrittenData.Int64())
	j.Policy.DownloadedBytes += counterDelta(j.previous.down, stats.BytesReadData.Int64())
	j.torrent = nil
	j.previous = observation{}
	j.upSpeed, j.downSpeed = 0, 0
	return metaErr
}

func uploadRate(bytes int64) rate.Limit {
	if bytes < 0 {
		return rate.Inf
	}
	return rate.Limit(bytes)
}

// SetUploadLimit changes the one shared client budget. Zero blocks payload upload; negative is unlimited.
func (e *Engine) SetUploadLimit(bytes int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return ErrClosed
	}
	e.limiter.SetLimit(uploadRate(bytes))
	return nil
}

// SetNetworkAvailable overrides automatic detection until the engine is closed.
// The core can provide authoritative OS network-change events, including on non-Windows hosts.
func (e *Engine) SetNetworkAvailable(available bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.network = &available
	for _, j := range e.tasks {
		j.previous.eligible = false
	}
}

func (e *Engine) SetSuspended(suspended bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.suspended = suspended
	for _, j := range e.tasks {
		j.previous.eligible = false
	}
}

func (e *Engine) isOnlineLocked() bool {
	if e.suspended {
		return false
	}
	if e.network != nil {
		return *e.network
	}
	return e.online()
}

func (e *Engine) run() {
	defer close(e.done)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.mu.Lock()
			if !e.closed {
				now, awake, online := e.now(), e.awake(), e.isOnlineLocked()
				for _, j := range e.tasks {
					e.sampleLocked(j, now, awake, online)
				}
				e.checkpointErr = e.saveLocked()
			}
			e.mu.Unlock()
		}
	}
}

func (e *Engine) sampleLocked(j *entry, now time.Time, awake time.Duration, online bool) {
	if j.torrent == nil {
		return
	}
	t := j.torrent
	if failure := j.failure.Load(); failure != nil {
		j.err = failure.err
	}
	if err := e.saveMetadataLocked(j); err != nil {
		j.err = err
	}
	if t.Info() != nil {
		j.Name, j.TotalBytes, j.CompletedBytes = t.Name(), t.Length(), t.BytesCompleted()
		j.Complete = t.Complete().Bool()
	}
	stats := t.Stats()
	cur := observation{at: now, awake: awake, up: stats.BytesWrittenData.Int64(), down: stats.BytesReadData.Int64()}
	cur.eligible = j.Complete && online && !e.suspended && !j.Policy.Paused && !j.Policy.ManuallyStopped && !j.Policy.AutoStopped && j.err == nil
	j.Policy.observe(j.previous, cur, j.Complete, j.TotalBytes)
	if dt := now.Sub(j.previous.at).Seconds(); dt > 0 {
		j.downSpeed = int64(float64(counterDelta(j.previous.down, cur.down)) / dt)
		j.upSpeed = int64(float64(counterDelta(j.previous.up, cur.up)) / dt)
	}
	j.previous = cur
	if j.err != nil || j.Policy.AutoStopped {
		if err := e.detachLocked(j); err != nil {
			j.err = err
		}
		return
	}
	e.enableLocked(j)
}

func (e *Engine) Snapshot() []Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	result := make([]Snapshot, 0, len(e.tasks))
	for _, j := range e.tasks {
		s := Snapshot{ID: j.ID, Name: j.Name, InfoHash: j.InfoHash, Folder: j.Folder, TotalBytes: j.TotalBytes, CompletedBytes: j.CompletedBytes,
			DownloadedBytes: j.Policy.DownloadedBytes, UploadedBytes: j.Policy.UploadedBytes, DownloadSpeed: j.downSpeed, UploadSpeed: j.upSpeed,
			SeedingSeconds: int64(j.Policy.SeedingTime / time.Second), Complete: j.Complete, Paused: j.Policy.Paused,
			ManuallyStopped: j.Policy.ManuallyStopped, AutoStopped: j.Policy.AutoStopped}
		if s.TotalBytes > 0 {
			s.Progress = min(1, float64(s.CompletedBytes)/float64(s.TotalBytes))
		}
		s.Status = StatusMetadata
		switch {
		case j.Policy.Paused:
			s.Status = StatusPaused
		case j.Policy.ManuallyStopped:
			s.Status = StatusStopped
		case j.Policy.AutoStopped:
			s.Status = StatusCompleted
		case j.err != nil:
			s.Status = StatusError
		case j.Complete:
			s.Status = StatusSeeding
		case j.TotalBytes > 0:
			s.Status = StatusDownloading
		}
		if j.err != nil {
			s.Error = j.err.Error()
		}
		if e.checkpointErr != nil {
			s.Error = errors.Join(j.err, e.checkpointErr).Error()
			s.Status = StatusError
		}
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (e *Engine) Close() error {
	e.cancel()
	e.mu.Lock()
	if e.closed {
		err := e.closeErr
		e.mu.Unlock()
		<-e.done
		return err
	}
	e.closed = true
	var errs []error
	now, awake, online := e.now(), e.awake(), e.isOnlineLocked()
	for _, j := range e.tasks {
		e.sampleLocked(j, now, awake, online)
		errs = append(errs, e.detachLocked(j))
	}
	errs = append(errs, e.client.Close()...)
	errs = append(errs, e.saveLocked(), e.completion.Close())
	e.closeErr = errors.Join(errs...)
	err := e.closeErr
	e.mu.Unlock()
	<-e.done
	return err
}

func (e *Engine) load() error {
	b, err := os.ReadFile(filepath.Join(e.dir, "tasks.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var state checkpoint
	if err = json.Unmarshal(b, &state); err != nil {
		return fmt.Errorf("corrupt download checkpoint: %w", err)
	}
	if state.Version != 1 {
		return fmt.Errorf("unsupported download checkpoint version %d", state.Version)
	}
	for _, saved := range state.Tasks {
		if saved.ID == "" || !filepath.IsAbs(saved.Folder) || saved.Policy.UploadedBytes < 0 || saved.Policy.DownloadedBytes < 0 || saved.Policy.SeedingTime < 0 || e.tasks[saved.ID] != nil {
			return errors.New("invalid task in download checkpoint")
		}
		if overlappingFolders(e.dir, saved.Folder) {
			return errors.New("checkpoint content folder overlaps state directory")
		}
		for _, other := range e.tasks {
			if overlappingFolders(other.Folder, saved.Folder) {
				return errors.New("checkpoint task folders overlap")
			}
		}
		e.tasks[saved.ID] = &entry{savedTask: saved}
	}
	return nil
}

func (e *Engine) saveLocked() error {
	state := checkpoint{Version: 1, Tasks: make([]savedTask, 0, len(e.tasks))}
	for _, j := range e.tasks {
		state.Tasks = append(state.Tasks, j.savedTask)
	}
	sort.Slice(state.Tasks, func(i, j int) bool { return state.Tasks[i].ID < state.Tasks[j].ID })
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err = atomicWrite(filepath.Join(e.dir, "tasks.json"), b); err != nil {
		return fmt.Errorf("save download checkpoint: %w", err)
	}
	return nil
}

func atomicWrite(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".checkpoint-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, writeErr := f.Write(b)
	err = errors.Join(writeErr, f.Sync(), f.Close())
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func overlappingFolders(a, b string) bool {
	if runtime.GOOS == "windows" {
		a, b = strings.ToLower(a), strings.ToLower(b)
	}
	for _, pair := range [][2]string{{a, b}, {b, a}} {
		rel, err := filepath.Rel(pair[0], pair[1])
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

type noStorage struct{}

func (noStorage) OpenTorrent(context.Context, *metainfo.Info, metainfo.Hash) (storage.TorrentImpl, error) {
	return storage.TorrentImpl{}, errors.New("per-task content folder required")
}
