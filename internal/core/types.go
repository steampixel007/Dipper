package core

import (
	"context"
	"errors"
	"time"
)

var (
	ErrClosed       = errors.New("core is closed")
	ErrNotFound     = errors.New("subscription not found")
	ErrTaskNotFound = errors.New("download task not found")
	ErrExists       = errors.New("subscription already exists")
	ErrStale        = errors.New("subscription changed during check")
)

type Anime struct{ ID, Title, Cover, Summary, Day string }
type Group struct {
	ID, Name string
	Count    int
}
type Resource struct {
	ID, Title, Published, Magnet, TorrentURL string
	Size                                     int64
}

type Rule struct {
	All  []string
	None []string
	Any  [][]string
}

type MatchResult struct {
	Matches bool
	Reason  string
}

type FeedFunc func(context.Context, string, string) ([]Resource, error)

// Ensure must be idempotent by Hash, durably accept a task before returning nil,
// and observe Context so Close can cancel an in-flight submission. Remove must
// forget the task by Hash and tolerate being called for a task it does not
// hold: startup recovery replays removals a crash interrupted, and the
// downloader may have already finished its half.
type Downloader interface {
	Ensure(DownloadTask) error
	Remove(hash string) error
}

type DownloadTask struct {
	Hash           string
	AnimeID        string
	SubscriptionID string
	Resource       Resource
	Directory      string
	Context        context.Context `json:"-"`
}

type Settings struct {
	DownloadRoot     string
	UploadLimitBytes int64
	// SeedMinutes is the seeding-time target: zero means seed without a time
	// limit (the ratio target still applies), negative means the default.
	SeedMinutes int64 `json:",omitempty"`
	// ProxyMode is one of the proxy package's mode constants; ProxyURL is
	// the address ProxyMode "custom" uses.
	ProxyMode string `json:",omitempty"`
	ProxyURL  string `json:",omitempty"`
}

type Options struct {
	Feed             FeedFunc
	Downloader       Downloader
	DownloadRoot     string
	Interval         time.Duration
	RetryBase        time.Duration
	RetryMax         time.Duration
	DisableScheduler bool
	Now              func() time.Time
}

type ItemStatus string

const (
	ItemPending    ItemStatus = "pending"
	ItemDownloaded ItemStatus = "downloaded"
	ItemExcluded   ItemStatus = "excluded"
	ItemSkipped    ItemStatus = "skipped"
)

type Item struct {
	Resource  Resource
	Hash      string
	Status    ItemStatus
	Read      bool
	Reason    string
	Explicit  bool
	Attempts  int
	LastError string
	NextRetry time.Time
}

type Subscription struct {
	ID          string
	Anime       Anime
	Group       Group
	Rule        Rule
	Directory   string
	Paused      bool
	Revision    uint64
	CreatedAt   time.Time
	LastCheck   time.Time
	LastSuccess time.Time
	NextCheck   time.Time
	LastError   string
	Failures    int
	Items       map[string]Item
}

type TaskRecord struct {
	Task        DownloadTask
	Accepted    bool
	Independent bool `json:",omitempty"`
	// Removing marks a removal the user confirmed whose downloader half may
	// not have finished; startup recovery completes it (recoverRemovals).
	Removing         bool `json:",omitempty"`
	RemoveRedownload bool `json:",omitempty"`
	Attempts         int
	LastError        string
	NextRetry        time.Time
}

type State struct {
	Settings      Settings
	Anime         []AnimeRecord
	Subscriptions []Subscription
	Tasks         []TaskRecord
}

type AnimeRecord struct {
	Anime     Anime
	Directory string
}
