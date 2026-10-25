# instance

`internal/instance` guarantees that one data directory has exactly one owner, and
forwards a later launch to the window that already owns it.

## What it owns

- The lock file that decides ownership.
- The endpoint a second process uses to reach the owner.
- Delivering an activation to the running window.

## Files

| File | Holds |
| --- | --- |
| `instance.go` | `Acquire`, `own`, `activate`, the listener and its handler, `Close` |
| `lock_windows.go`, `lock_other.go` | The platform's advisory file lock |

## Why it exists

Two processes on one data directory corrupt it. The SQLite file is written by
`core`, and each task folder under the download root is written by the
downloader; neither is designed for a second writer.

The lock is therefore taken **before** the database is opened. `main` resolves
the platform data directory (or the explicit `--data` directory), then calls
`Acquire` before opening persistent state. A failure to acquire is a startup
error, not a degraded mode.

## How ownership works

`Acquire` creates or opens `instance.lock` in the data directory, then calls the
platform's `tryLock`:

- On Windows, an exclusive lock on the file handle.
- Elsewhere, `flock` with `LOCK_EX|LOCK_NB`.

If the lock succeeds, this process owns the directory and `own` starts a listener
on `127.0.0.1:0` — a loopback address and an ephemeral port, never a fixed port
and never a non-loopback interface.

The listener's address, a 32-byte random token, and the process ID are written to
`instance.json` through a temporary file and a rename, so a reader never sees a
half-written endpoint.

If the lock fails, another process owns the directory, and `Acquire` reads
`instance.json` and calls `activate`.

## The activation exchange

`activate` validates the endpoint before dialling it:

- the address must parse, and its host must be exactly `127.0.0.1`;
- the token must be exactly 64 hex characters.

Then it dials with a 200 ms connect timeout, sends the token, and waits for a
`true` acknowledgement within a one-second deadline.

The owner's handler accepts the connection, reads at most 256 bytes, and compares
the token with `subtle.ConstantTimeCompare`. Only a matching token pushes an
activation into the channel and acknowledges. The channel has capacity 1 with a
non-blocking send, so a burst of launches collapses into one activation.

The constant-time comparison is not decoration: the token is the only thing
distinguishing a real launch from any other local process that can reach the
port.

## What `Acquire` returns

| Result | Means |
| --- | --- |
| `(*Instance, nil)` | This process owns the directory and must run the window |
| `(nil, nil)` | Another process owns it and acknowledged the activation; exit quietly |
| `(nil, err)` | Ownership could not be established, or activation failed; report and exit |

The `(nil, nil)` case is the one that must never fall through to opening the
database. The package comment states this directly: it never proceeds to open
the application's databases when activation fails.

## Retry and cancellation

If activation fails, `Acquire` retries every 50 ms until the context ends. The
owner may still be starting up: the lock is held before the listener exists, so
there is a window where the file is locked but `instance.json` is not yet
written. Retrying covers that window without a fixed sleep.

The context's cancellation becomes a user-facing message that the app is running
in the background but its window could not be woken.

## Tests

`instance_test.go` runs two `Acquire` calls against one directory and asserts
that exactly one returns an owner while the other gets an activation. Points a
change is expected to prove:

- A second `Acquire` on the same directory does not return an owner.
- A wrong token is refused and produces no activation.
- An endpoint whose host is not `127.0.0.1` is refused before dialling.
- The activation channel receives one value for several concurrent launches.
- `Close` removes `instance.json` and releases the lock, so the next `Acquire`
  becomes the owner.

## Invariants a change must preserve

- The lock is taken before anything else touches the data directory.
- The listener binds loopback only.
- The endpoint file is written by rename, never in place.
- The token is compared in constant time.
- A failed activation never lets the process continue to open the database.
