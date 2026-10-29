package core

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"dipper.local/dipper/internal/proxy"
	_ "modernc.org/sqlite"
)

type Core struct {
	db        *sql.DB
	opts      Options
	mu        sync.Mutex
	delivery  chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	closed    bool
	started   bool
	closeDone chan struct{}
	closeErr  error
	wake      chan struct{}
	active    map[*subscriptionOperation]struct{}
	removing  map[string]int
}

type subscriptionOperation struct {
	id     string
	hash   string
	cancel context.CancelFunc
}

func (c *Core) registerLocked(ctx context.Context, id string) (context.Context, func()) {
	return c.registerTaskLocked(ctx, id, "")
}

func (c *Core) registerTaskLocked(ctx context.Context, id, hash string) (context.Context, func()) {
	child, cancel := context.WithCancel(ctx)
	op := &subscriptionOperation{id: id, hash: hash, cancel: cancel}
	c.active[op] = struct{}{}
	return child, func() { cancel(); c.mu.Lock(); delete(c.active, op); c.mu.Unlock() }
}

func (c *Core) cancelSubscriptionLocked(id string) {
	for op := range c.active {
		if op.id == id {
			op.cancel()
		}
	}
}

func (c *Core) watchSubscription(ctx context.Context, id string, revision uint64) (context.Context, func(), error) {
	var child context.Context
	var cleanup func()
	err := c.transaction(ctx, func(_ *sql.Tx, s *diskState) error {
		sub, ok := s.Subscriptions[id]
		if !ok {
			return ErrNotFound
		}
		if sub.Paused || sub.Revision != revision {
			return ErrStale
		}
		child, cleanup = c.registerLocked(ctx, id)
		return nil
	})
	if err != nil && cleanup != nil {
		cleanup()
	}
	return child, cleanup, err
}

type diskState struct {
	Settings      Settings
	Anime         map[string]AnimeRecord
	Subscriptions map[string]Subscription
}

func Open(path string, opts Options) (*Core, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Interval <= 0 {
		opts.Interval = 15 * time.Minute
	}
	if opts.RetryBase <= 0 {
		opts.RetryBase = time.Minute
	}
	if opts.RetryMax <= 0 {
		opts.RetryMax = time.Hour
	}
	useDefaultRoot := opts.DownloadRoot == ""
	if useDefaultRoot {
		var err error
		opts.DownloadRoot, err = DefaultDownloadRoot()
		if err != nil {
			return nil, err
		}
	}
	root, err := filepath.Abs(opts.DownloadRoot)
	if err != nil {
		return nil, err
	}
	opts.DownloadRoot = root
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithCancel(context.Background())
	c := &Core{db: db, opts: opts, ctx: ctx, cancel: cancel, delivery: make(chan struct{}, 1), closeDone: make(chan struct{}), wake: make(chan struct{}, 1), active: make(map[*subscriptionOperation]struct{}), removing: make(map[string]int)}
	_, err = db.Exec(`PRAGMA busy_timeout=5000; PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL;
		CREATE TABLE IF NOT EXISTS core_state (id INTEGER PRIMARY KEY CHECK(id=1), body BLOB NOT NULL);
		CREATE TABLE IF NOT EXISTS core_tasks (hash TEXT PRIMARY KEY, body BLOB NOT NULL);
		PRAGMA user_version=1;`)
	if err == nil {
		initial := diskState{Settings: Settings{DownloadRoot: root, UploadLimitBytes: 1024 * 1024}, Anime: map[string]AnimeRecord{}, Subscriptions: map[string]Subscription{}}
		body, _ := json.Marshal(initial)
		_, err = db.Exec(`INSERT OR IGNORE INTO core_state(id,body) VALUES(1,?)`, body)
	}
	if err != nil {
		cancel()
		db.Close()
		return nil, err
	}
	if useDefaultRoot {
		home, homeErr := os.UserHomeDir()
		if homeErr == nil {
			legacy := filepath.Join(home, "Downloads", "Dipper")
			err = c.transaction(ctx, func(_ *sql.Tx, state *diskState) error {
				if filepath.Clean(state.Settings.DownloadRoot) == legacy {
					state.Settings.DownloadRoot = root
				}
				return nil
			})
			if err != nil {
				cancel()
				db.Close()
				return nil, err
			}
		}
	}
	return c, nil
}

func (c *Core) operation(ctx context.Context) (context.Context, func(), error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, nil, ErrClosed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	child, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(c.ctx, cancel)
	if c.ctx.Err() != nil {
		cancel()
	}
	c.wg.Add(1)
	return child, func() { stop(); cancel(); c.wg.Done() }, nil
}

func loadState(ctx context.Context, tx *sql.Tx) (diskState, error) {
	var body []byte
	var state diskState
	if err := tx.QueryRowContext(ctx, `SELECT body FROM core_state WHERE id=1`).Scan(&body); err != nil {
		return state, err
	}
	err := json.Unmarshal(body, &state)
	return state, err
}

func (c *Core) transaction(ctx context.Context, fn func(*sql.Tx, *diskState) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	s, err := loadState(ctx, tx)
	if err != nil {
		return err
	}
	if err = fn(tx, &s); err != nil {
		return err
	}
	body, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE core_state SET body=? WHERE id=1`, body); err != nil {
		return err
	}
	return tx.Commit()
}

func taskByHash(ctx context.Context, tx *sql.Tx, hash string) (TaskRecord, error) {
	var body []byte
	var r TaskRecord
	err := tx.QueryRowContext(ctx, `SELECT body FROM core_tasks WHERE hash=?`, hash).Scan(&body)
	if err == nil {
		err = json.Unmarshal(body, &r)
	}
	return r, err
}

func saveTask(ctx context.Context, tx *sql.Tx, r TaskRecord) error {
	body, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO core_tasks(hash,body) VALUES(?,?) ON CONFLICT(hash) DO UPDATE SET body=excluded.body`, r.Task.Hash, body)
	return err
}

func readTasks(ctx context.Context, tx *sql.Tx) ([]TaskRecord, error) {
	rows, err := tx.QueryContext(ctx, `SELECT body FROM core_tasks ORDER BY hash`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []TaskRecord
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		var r TaskRecord
		if err := json.Unmarshal(b, &r); err != nil {
			return nil, err
		}
		tasks = append(tasks, r)
	}
	return tasks, rows.Err()
}

func (c *Core) Snapshot() (State, error) {
	var out State
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return out, ErrClosed
	}
	tx, err := c.db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	s, err := loadState(context.Background(), tx)
	if err != nil {
		return out, err
	}
	err = func() error {
		out.Settings = s.Settings
		for _, a := range s.Anime {
			out.Anime = append(out.Anime, a)
		}
		for _, sub := range s.Subscriptions {
			out.Subscriptions = append(out.Subscriptions, sub)
		}
		var err error
		out.Tasks, err = readTasks(context.Background(), tx)
		return err
	}()
	sort.Slice(out.Anime, func(i, j int) bool { return out.Anime[i].Anime.ID < out.Anime[j].Anime.ID })
	sort.Slice(out.Subscriptions, func(i, j int) bool { return out.Subscriptions[i].ID < out.Subscriptions[j].ID })
	return out, err
}

func (c *Core) GetSettings() (Settings, error) { s, e := c.Snapshot(); return s.Settings, e }
func (c *Core) Subscriptions() ([]Subscription, error) {
	s, e := c.Snapshot()
	return s.Subscriptions, e
}
func (c *Core) ListTasks() ([]TaskRecord, error) { s, e := c.Snapshot(); return s.Tasks, e }
func (c *Core) Anime() ([]AnimeRecord, error)    { s, e := c.Snapshot(); return s.Anime, e }
func (c *Core) SetSettings(s Settings) error     { return c.UpdateSettings(context.Background(), s) }

func (c *Core) UpdateSettings(ctx context.Context, settings Settings) error {
	if strings.TrimSpace(settings.DownloadRoot) == "" || settings.UploadLimitBytes < 0 {
		return errors.New("invalid download root or upload limit")
	}
	if err := proxy.Validate(settings.ProxyMode, settings.ProxyURL); err != nil {
		return err
	}
	root, err := filepath.Abs(settings.DownloadRoot)
	if err != nil {
		return err
	}
	settings.DownloadRoot = root
	return c.transaction(ctx, func(_ *sql.Tx, s *diskState) error { s.Settings = settings; return nil })
}

func safeName(title string) string {
	name := strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, title)
	name = strings.Trim(name, " .")
	if name == "" {
		name = "番剧"
	}
	stem := strings.ToUpper(strings.Split(name, ".")[0])
	if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || (len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '1' && stem[3] <= '9') {
		name = "_" + name
	}
	if len(name) > 160 {
		name = name[:160]
		for !utf8.ValidString(name) {
			name = name[:len(name)-1]
		}
	}
	return name
}

func animeDirectory(s *diskState, a Anime) string {
	if existing, ok := s.Anime[a.ID]; ok {
		return existing.Directory
	}
	name := safeName(a.Title)
	dir := filepath.Join(s.Settings.DownloadRoot, name)
	for _, v := range s.Anime {
		if strings.EqualFold(v.Directory, dir) {
			h := sha256.Sum256([]byte(a.ID))
			dir = filepath.Join(s.Settings.DownloadRoot, fmt.Sprintf("%s [%s]", name, hex.EncodeToString(h[:])))
			break
		}
	}
	s.Anime[a.ID] = AnimeRecord{Anime: a, Directory: dir}
	return dir
}

func (c *Core) signal() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func (c *Core) Close() error {
	c.mu.Lock()
	if c.closed {
		done := c.closeDone
		c.mu.Unlock()
		<-done
		return c.closeErr
	}
	c.closed = true
	c.cancel()
	c.mu.Unlock()
	c.wg.Wait()
	c.closeErr = c.db.Close()
	close(c.closeDone)
	return c.closeErr
}
