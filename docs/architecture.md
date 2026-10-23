# Architecture decisions

This page records the reasons for choices that still shape Dipper. The current
behavior and invariants live in the [subsystem pages](subsystems/README.md);
when this page and the code disagree, update this page in the same change.

## Native interface in Go

Dipper uses MyGo Native UI and Go for the whole window. `cmd/dipper/main.go`
creates a window with `Content: ui.View(s.view)`, so MyGo draws the interface
instead of loading a page through its WebView window mode. That keeps the UI and
application logic in one language and avoids a frontend build and binding layer.

MyGo itself supports both modes and includes WebView2 code on Windows. Finding
WebView2 symbols in the executable therefore does not establish that Dipper's
window uses a WebView; the `Content` window option determines its rendering path.
The confirmed choice is MyGo Native UI + Go. A different UI stack would be a
product and maintenance decision, not an incidental dependency update.

## Pure Go builds and SQLite

CI and `scripts/build.ps1` set `CGO_ENABLED=0`. This lets Go cross-compile the
app without a target C compiler. A successful cross-compile only establishes
that it builds; graphics, tray behavior and installation still need tests on
real macOS and Linux machines.

The database driver is `modernc.org/sqlite`, imported in
`internal/core/store.go`. A cgo-based SQLite driver would conflict with those
builds. `github.com/anacrolix/torrent` also keeps the BT client in Go; a
libtorrent binding would add native library and toolchain requirements. These
requirements are the reason for the dependency choices, beyond their current
versions in `go.mod`.

## BT engine boundary

`core.Downloader` in `internal/core/types.go` exposes
`Ensure(DownloadTask) error`. The engine in `internal/downloader` implements
that contract, and `cmd/dipper` wires the two together. Core does not import
the BT library.

The boundary lets the engine change while preserving the rule that a task is
identified by its BT info hash. `Ensure` must accept a task durably and be
idempotent for that hash; its context governs admission. Once admitted, the
engine owns the transfer lifetime. The precise contract and its tests are in
the [downloader page](subsystems/downloader.md). An engine replacement would
still have to meet them, including pause, resume and the shared upload limit.

## One matcher for previews and automatic downloads

`core.Match` in `internal/core/rules.go` evaluates rules against original
release titles. `cmd/dipper/model.go` uses it for the visible preview;
`internal/core/engine.go` uses it for subscription decisions. Reusing the same
function prevents the preview from promising a different result from the
scheduler.

Normalization folds ASCII case and full-width ASCII, but does not equate
simplified and traditional Chinese. Recognized episode and subtitle labels
help people read a title; they do not silently change a saved rule. The
[core page](subsystems/core.md) describes the rule and its invariants.

## Source items and BT content have different identities

A source episode ID identifies a catalog item; a verified BT info hash
identifies download content. Two different releases of the same episode can
both be downloaded, while repeated references to the same hash share one
download task. An episode number is neither kind of identity.

`internal/core/hash.go` reads a v1 hash from a magnet or derives it from a
torrent's `info` dictionary. Core can keep an item pending until an opaque
torrent URL has been resolved. The [source](subsystems/source.md) and
[core](subsystems/core.md) pages spell out the boundaries between these IDs.

## Complete history before RSS updates

Mikan's `ExpandEpisodeTable` accepts an absolute `take` count, not an offset or
cursor. `internal/source/html.go` increases `take` until the endpoint returns
fewer rows than requested; otherwise it reports incomplete history. RSS holds
at most 100 recent entries, so it cannot establish all existing releases for a
long-running anime. A new subscription uses the history list; scheduled checks
use RSS for later updates. This is covered by `TestHistory1522AndRSS100`.

## One owner of persistent state

Core owns one SQLite database. `internal/core/store.go` limits it to one
connection and serializes mutations through transactions; this avoids
interleaved read-modify-write cycles. Its two tables hold the current model and
durable download tasks. The [core page](subsystems/core.md) describes the
schema and the write ordering.

`internal/instance` also keeps one process per data directory and forwards a
second launch to the existing window. Two processes must not independently
write the same database and BT state. The [instance page](subsystems/instance.md)
describes the lock and its recovery behavior.

The product requirements remain in [PRD-v1.md](../PRD-v1.md), and the visual
baseline in [DESIGN-v1.md](../DESIGN-v1.md). Installer packaging, signing
and updates have not been implemented; their current limits are in the
[README](../README.md#verification-and-boundaries).
