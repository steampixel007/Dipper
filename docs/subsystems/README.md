# Subsystem references

Each page here describes one subsystem as it is built: what it owns, which files
carry it, the invariants a change must preserve, and how it is tested. Read the
page for a subsystem before changing it, and check the source it links — a page
that has drifted from the code is a bug in the page.

| Subsystem | Owns | Source |
| --- | --- | --- |
| [core](core.md) | The subscription model, the SQLite state, matching rules, hash identity, the scheduler and the delivery outbox | `internal/core/` |
| [downloader](downloader.md) | The embedded BT engine, transfer policy, seeding limits, storage layout and resume | `internal/downloader/` |
| [source](source.md) | Fetching the anime catalog and a group's releases, and parsing them out of HTML | `internal/source/` |
| [instance](instance.md) | One owner per data directory, and forwarding a second launch to the running window | `internal/instance/` |
| [gui](gui.md) | The native window, its pages, and the state it renders | `cmd/dipper/` |

## How to describe a change to a subsystem

A change to a documented subsystem says which page it touches and updates that
page in the same change. The description names the behavior a user sees, not the
functions that changed. If a page's claim no longer holds, fix the page rather
than leaving a second source of truth.

## Cross-cutting rules

These hold in every subsystem and are not repeated on each page:

- A read, parse or hash that fails means *unknown*, never *empty* or *equal*.
- Anything that can block does not run under a lock its reader needs.
- A default never overrides a value the user set.
- A test that touches the network serves the vendor from `httptest` with bytes
  saved from the real response.
- Platform splits (`*_windows.go` / `*_other.go`) change together.
