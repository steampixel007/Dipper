# source

`internal/source` reads the public Mikan catalog, a group's episode history, and
RSS feeds, and parses them into the types `core` consumes.

## What it owns

- HTTP access to the source, with shared concurrency and per-origin pacing.
- Turning HTML and RSS into `Anime`, `Group` and `Resource`.
- Deciding when a response is unusable, and saying why.

## Files

| File | Holds |
| --- | --- |
| `client.go` | `Client`, the rate limit, redirect and size limits, the challenge check, `Cover` |
| `html.go` | Catalog, detail page and episode-history parsing |
| `feed.go` | RSS parsing |

## Types and their traps

`Resource.ID` is the **source's episode ID**, not a verified torrent infohash.
The two are different namespaces and must not be mixed. `core` derives identity
from the hash when it has one; the source ID is only the catalog's key.

`Resource.Published` keeps the source's timezone-less text as written. It is
never parsed into a `time.Time`, because the source does not say which zone it
means and guessing would shift every release by hours.

`Resource.Size` is approximate for HTML and zero when the source omits it. A
zero size means *unknown*, not *empty* — the downloader must not treat it as a
zero-byte target. Source sizes may use thousands separators, as in
`1,017.6 MB`; only valid three-digit grouping is accepted, and the commas are
removed only after that check. A grouped size parses to the same approximate
byte value as the ungrouped spelling.

`Group.Count` is `-1` when the detail page only shows a truncated preview, which
is different from `0`. `Group.Preview` retains the resources parsed from that
detail-page table: when `Count >= 0` it is the group's complete history, and
when `Count == -1` it is a truncated preview that must not be submitted as
complete history.

## The client

One `Client` is shared by the catalog, subscriptions and covers. Reusing one
client shares transport admission and the cover slot budget; pacing and
server-requested cooldowns are tracked per origin.
`New` uses `https://mikanani.me` (line two) when no origin is supplied.
The client's transport resolves its proxy through `internal/proxy`, so the
setting chosen in the window applies to catalog, history, RSS and cover
requests alike; loopback addresses stay direct.
`Retarget` swaps the origin under the client's lock while keeping shared
transport admission and in-flight requests, so switching lines does not
multiply the request rate. Each origin keeps its own pacing and cooldown state.

Document reads use a single slot for the whole operation and start at least
1.5 seconds apart. Cover reads use six separate slots and can start up to six
attempts immediately when the budget is full. Each cover attempt spends one
start, and the budget restores one every 750 milliseconds up to six.
Both request types honor `Retry-After`. A cover `403`, `429`, or challenge also
pauses covers for at least one minute. At the end of a cooldown, one cover can
start immediately, with the remaining starts replenished at the normal pace.
The limiter rechecks its deadline after waiting so a response arriving during
the wait can defer the next attempt.

`CoverWhenReady` lets the UI withdraw a queued cover when a higher-priority
cover needs the slot. Its non-blocking callback runs at admission, after the
slot and interval wait but before spending a start from the cover budget.

An EOF while receiving headers or reading the body gets one retry. Both attempts
share transport admission, the 30-second operation timeout and per-origin
pacing; an incomplete body is closed and discarded before retrying. If either
attempt encounters an EOF and its context has already ended, the operation
returns the context error.
HTTP statuses, challenges and parsing failures do not trigger this retry.

Redirects are followed at most five times, and every redirect target must still
be an absolute `http`/`https` URL with no userinfo. This is what stops a
redirect into `file:` or into a URL carrying credentials.

Response bodies are capped per call: 64 MiB for a document, 5 MiB for a cover.
The cap is applied twice — once against `ContentLength` and once against the
bytes actually read through an `io.LimitReader` — because a lying
`Content-Length` must not be able to allocate unbounded memory.

Catalog cover URLs carry `width` and `height` parameters that crop full poster
art into square thumbnails. `Cover` removes both through `OriginalCoverURL`
before requesting. The response is then the complete artwork; displaying it at a fixed
square size would undo this at the last step.

A non-200 response becomes `HTTPError`, carrying `RetryAfter` parsed from the
header in either its seconds or its HTTP-date form. `RetryAfter` is pushed into
the origin's pacing state so later requests wait it out. For covers, a `403`,
`429`, or challenge also defers new cover attempts for at least a minute, even
if the response carries no `Retry-After`. The longest pending deadline wins.

## The challenge check

`challenge` looks for Cloudflare's interstitial markers. Its comment records the
one thing that makes this safe: Cloudflare's analytics script
(`/cdn-cgi/challenge-platform/`) is present on ordinary pages too, so the check
matches the markers that only appear on a challenge page, not the analytics
prefix alone.

A detected challenge is `ErrChallenge`, never a parse failure. The distinction
matters: a challenge is transient and should be retried later, while a parse
failure means the page structure changed and retrying will not help.

## Parsing

Detail pages provide an optional synopsis in the direct `.header2-desc` child
of the desktop `.central-container`. It is normalized as plain text and kept
in `Anime.Summary`; descriptions inside individual resource entries are not
anime synopses. A missing synopsis is valid and does not block subtitle groups.

The catalog and history parsers are positional: they read a table's rows and
cells by their place in the markup. This is why `testdata/` holds saved pages —
a parser change is checked against the real markup it must read, not against a
fixture written from the same assumption as the parser.

`document` guards one specific failure: the HTML parser repairs a truncated
table, so a cut-off response would otherwise look like a legitimately short
final history page. If an episode table is present but the body has no closing
`</table>`, the response is `ErrStructure` rather than a short history.

Season and weekday names arrive in Chinese and are mapped to internal values.
Those literals are data from the source, not UI strings, and must not be
translated.

## Errors

| Error | Means | Retry helps |
| --- | --- | --- |
| `ErrStructure` | The response parsed but is not the shape expected | No — the page changed |
| `ErrChallenge` | An access challenge was served | Yes, later |
| `ErrIncompleteHistory` | The history cannot be confirmed complete | Maybe |
| `ErrNotFound` | The anime or group does not exist | No |
| `ErrResponseTooLarge` | The body exceeded its cap | No |
| `*HTTPError` | A non-200 status | Depends on the status and `RetryAfter` |

The difference between "unknown" and "empty" is load-bearing here: a history
that cannot be confirmed complete is `ErrIncompleteHistory`, not an empty list,
because an empty list would silently mark every past release as new.

## Tests

`internal/source` is the one package that talks to a real host. Its tests must
not: a test that needs a page serves it from `httptest` out of
`testdata/*.html`, captured from the real source. A test that genuinely needs the
live host is skipped under `testing.Short()` and says in the PR that it was not
run.

Points a change is expected to prove:

- A truncated episode table is `ErrStructure`, not a short history.
- A challenge page is `ErrChallenge`, not `ErrStructure`.
- A redirect to a non-HTTP scheme is refused.
- A `Content-Length` larger than the cap is refused before reading.
- `Retry-After` in both forms moves the origin's pacing and cooldown forward.
- An omitted size parses as zero, and zero is distinguishable from a real zero.
- A grouped size parses like its ungrouped spelling, and malformed grouping is
  refused rather than silently read as a different number.

## Invariants a change must preserve

- One `Client` is shared; pacing is per origin and concurrency is per client,
  not per call.
- Every request URL is validated as absolute HTTP(S) with no userinfo.
- A body cap is enforced against both the declared and the actual length.
- A challenge is never reported as a parse failure, and a parse failure is never
  retried as if it were transient.
- Source-side Chinese literals stay untranslated.
- Parsers are checked against `testdata/` markup captured from the real source.
