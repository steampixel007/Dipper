# Notes for coding agents

This file does one thing: it tells you which document to read before you work.
The rules themselves are not here; they are in the files this points at. Do not
add rules to this file.

## Before changing code

Read [LESSONS.md](LESSONS.md): what recent merged work got wrong, and the rule
that would have caught it.

## Writing and reviewing code

Follow [docs/code-standards.md](docs/code-standards.md). The acceptance criteria
for a change are there, including the rule that a test must fail without the
change and how to prove it.

## By subsystem

Read the page for a subsystem before changing it. Each page names the files that
carry it, the invariants a change must preserve, and what its tests prove.

| To change | Read first |
| --- | --- |
| The subscription model, SQLite state, matching, hash identity, the scheduler, the outbox | [docs/subsystems/core.md](docs/subsystems/core.md) |
| The BT engine, piece storage, transfer policy, the shared upload limit | [docs/subsystems/downloader.md](docs/subsystems/downloader.md) |
| Fetching and parsing the catalog, a group's history, RSS, covers | [docs/subsystems/source.md](docs/subsystems/source.md) |
| The single-instance lock and window activation | [docs/subsystems/instance.md](docs/subsystems/instance.md) |
| The window, its pages and the state it renders | [docs/subsystems/gui.md](docs/subsystems/gui.md) |

[docs/subsystems/README.md](docs/subsystems/README.md) is the index, and lists the
cross-cutting rules that hold in every subsystem.

## Platform splits

`*_windows.go` and `*_other.go` are two implementations of one thing. Changing
one means reading the other. The build must pass on all three platforms; see
`.github/workflows/check.yml`.

## Tests that touch the source

`internal/source` is the only package that talks to a real host, and its tests
must not. Serve the page from `httptest` out of `testdata/`, captured from the
real site. A test that genuinely needs the live host is skipped under
`testing.Short()` and says so in the change description.

## Before committing

Run `scripts/build.ps1 -Test`, which does `go test -count=1 -timeout 5m ./...`
and `go vet ./...`.

A new test must prove it catches the bug. Judge that with
[cmd/redcheck](cmd/redcheck) rather than by eye: it tells a real failure from a
compile error, a skip, a timeout or a panic. It does not decide which code to
put back — that judgment is yours.

## Language

Documentation and code comments are English. User-visible strings stay Chinese:
they are product copy, not documentation. Chinese literals read from the source
site (season and weekday names, page labels) are data and are never translated.
See [docs/code-standards.md](docs/code-standards.md#language).
