# Code standards

These rules come from work that had to be done twice. Each one says what a
reviewer looks at, and the failure that put it here. A change is done only when
it shows each of them.

## What a change contains

| Rule | What a reviewer looks at |
| --- | --- |
| A change does only what its goal needs. | Every behavior change has a reason. Unrelated refactors, extra features and policy changes go in a change of their own. |
| A fix covers exactly the cases it means to. | Say which cases change and which neighboring ones do not. Different errors are not handled as one. |
| A change holds on the whole path. | Follow it from the user's input to the end result, through the callers, the state it changes and the side effects. One correct function does not make the whole operation correct. |
| A fix reaches every sibling of the bug. | Grep for the same shape: another branch of the same operation, another caller of a changed predicate, another message naming the same endpoint. Fixing only the case in the report is the main source of rework. |
| A change re-derives everything built on what it changes. | When a change alters a formula, a list, a layout breakpoint or the timing of an async step, find every caller, saved copy and test that assumed the old meaning, and run it. |
| A failed read, parse or hash means *unknown*. | Never *empty*, *none* or *equal*. Keep what was read before; copy a file that does not parse aside before writing over it; fix the shared reader rather than adding a stricter one beside it. A list that guards access fails closed when its file cannot be read. Test with the file unreadable. |
| Another app's values come from that app. | When writing another app's file or speaking for it, take each field's type and value from that app's own data, binary, merge code or docs, not only from its field names. |
| Nothing that can block runs under a lock its reader needs. | Do not send on a channel or call anything that can block while holding a lock the receiver takes first. Set shared state before the handoff that lets another goroutine see it. A struct copied by value that holds a map is not a copy. A change that starts a goroutine or a timer runs `-race` before it is pushed. |
| A default never overrides a value the user set. | Before writing a key, list everywhere the setting is read from, and test with a user-set value present. A migration that rewrites a setting names the users who set it themselves and leaves their value alone. |
| Checks cover the edges the change touches. | For numbers, check the boundaries. For paths, check each platform. For concurrency, check races and stale state. Pick checks by risk. |

## Tests

| Rule | What a reviewer looks at |
| --- | --- |
| A fix comes with a test that fails without it. | Put the old code back and run the test: it must fail with a real `FAIL` that names the behavior. Judge it with [cmd/redcheck](../cmd/redcheck), not by eye. A compile error, a skipped test, a timeout or a panic does not count. |
| A test never reaches the network. | A vendor's page or API is served from `httptest`, built from bytes saved from the real response. A dependency comes from a version pinned in `go.sum`, not from the registry's latest. Break the change and watch where the test's requests go. |
| Tests use real inputs. | Fixtures look like real requests, real serialized output and real files on disk, including missing fields, empty values and defaults. `internal/source/testdata/` holds pages captured from the real site for exactly this. |
| Tests check behavior, not implementation. | Assert what the user or caller needs. A formatting change and an internal refactor should not break a test, unless exact bytes are the requirement. |
| A test that fails on the baseline is a bug to fix now. | "It fails the same on `HEAD`" is not a baseline, and neither is "it fails under the full run but passes alone". Fix it at its cause, or open an issue naming the cause in a change of its own, before building on top of it. Before calling a Go test flaky, run `go test -race -count=20 -run X`. |
| Tests stay out of the real machine. | Use `t.TempDir()`. Never read or write the user's real data directory, preferences or home. A test that needs the live source is skipped under `testing.Short()`. |
| Whatever a test starts ends when the test does. | A fake process, a goroutine or an async refresh is stopped or waited for before the test returns, or it writes into the next test's folder ("directory not empty") or spins for days. |
| A timing test leaves room on both sides. | Never bound a test by wall-clock time a loaded machine can exceed, and keep a threshold's fixture far from every threshold it has. Reproduce a timing failure by injecting the delay (a held write, a late first arrival), not by rerunning until it fails. List every timing assumption in the test when fixing one. |

## Acceptance criteria

A change is done only when it shows each of the following. Say which ones were
run and what they printed. A check that was not run is listed as not run, never
left out.

1. **A test that fails without the change.** Put the old code back and run the
   new test. It must fail with a real `FAIL` that names the behavior. A compile
   error or a skipped test does not count. Say which test fails and what it
   failed with. Then restore the change and check the test passes.

   Judge it with `redcheck`, not by eye:

   ```
   # 1. With the change, confirm the test passes.
   go run ./cmd/redcheck -test '^TestFoo$' -expect pass

   # 2. Put the old code back by hand. Which code is yours to decide;
   #    the tool does not choose it.

   # 3. Judge whether this red counts.
   go run ./cmd/redcheck -test '^TestFoo$' -expect fail

   # 4. Restore the change.
   ```

   Exit codes: `0` real red (or passed), `1` not red (or did not pass), `2`
   false red (compile error, skip, timeout, panic), `3` usage or environment
   error. A false red usually means the code put back changed an interface —
   put back a different place and try again.

2. **Break it on purpose.** Break one place at a time and check that the new
   test catches each break. Record which tests failed and what they printed.
   Cover the branches this change touches, not random lines.

3. **The build and the whole suite.** Run `scripts/build.ps1 -Test`, which does
   `go test -count=1 -timeout 5m ./...` and `go vet ./...`. Also:

   - `gofmt -l cmd internal` prints nothing
   - `CGO_ENABLED=0 GOOS=linux go build ./cmd/dipper`
   - `CGO_ENABLED=0 GOOS=darwin go build ./cmd/dipper`

   Run `./...`, not the packages you think you touched.

4. **Real use where possible.** A change to the downloader, the source reader or
   the single-instance path is also checked by running the app against a real
   data directory. If that is not possible, say so. What was not tried is not
   built on and is not told to users as working.

5. **Platforms.** A change to `*_windows.go` or `*_other.go` is checked on both.
   A cross-compile is not a test; it only shows that it compiles.

6. **The UI rules** below hold.

## Reviewing a change

- **Run it.** Pull it, merge it onto current `HEAD`, and run the acceptance
  criteria. Reading the diff alone is not a review.
- **Break it on purpose.** Revert the fix, or break the code it guards in
  several ways, and check that the new tests catch each break.
- **Check the source, not the change's account of it.** When it depends on
  upstream behavior, read the upstream source or a real response.
- **Check each platform.** An `int` conversion that is safe on arm64 can wrap on
  amd64.
- **Follow the whole path.** A value saved under one id and read under another
  is the most common class of bug here.
- **Classify narrowly.** A mark or an error class covers only the case it
  describes.
- **A preview writes nothing.** A dry run, plan or preview has no side effects.
- **Do not reverse a deliberate decision.** Interaction trade-offs belong to the
  project owner; a change's goal does not include quietly changing one.
- **Docs match the source.** A reference points at code that exists, and every
  link resolves.
- **Keep the change to its goal.** Unrelated tests and refactors go in a change
  of their own.

## Commits

A commit subject names the area it changes, then says what the user now sees,
written as plain behavior. The body says how it was verified, which test fails
without the change, and what was not run.

The record of what was run lives in the commit body, where a later review can
read it. A large change with no body cannot be reviewed afterwards.

## Go

- Write `any`, not `interface{}`.
- A comment says what the code could not: the intent, an unobvious constraint,
  why this and not that. Delete comments that restate the code, record change
  history, or mark positions.
- An error carries context: `fmt.Errorf("...: %w", err)`.

## Language

- Documentation and code comments are English.
- A string the user sees stays Chinese. It is product copy, not documentation,
  and it is not translated.
- A Chinese literal read from the source site — a season name, a weekday name, a
  page label such as `放送日期：` — is data. It is never translated, because
  translating it would stop it matching the page.
- An identifier, a config key, a CLI flag and a log field stay ASCII.

## UI

- Every string the user sees goes through one place, so translations can be
  added later.
- A click never moves the page's scroll position.
- Mark state with a dot or a swatch, not a colored left-border stripe.
