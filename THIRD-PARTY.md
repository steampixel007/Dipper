# Third-party notices

Dipper is licensed under the MIT License; see [LICENSE](LICENSE). Release
binaries also contain the Go modules listed below. Each module stays under its
own license, and those licenses continue to apply to the module's code.

This file lists the union of the Windows, macOS, and Linux release builds
(`CGO_ENABLED=0`), plus the extra modules that only CGO-enabled builds link.
Modules that appear only in the module graph are omitted. Run the command below
once per target platform to see that platform's subset:

```sh
CGO_ENABLED=0 go list -deps -f '{{if .Module}}{{.Module.Path}} {{.Module.Version}}{{end}}' ./cmd/dipper
```

## Direct dependencies

| Module | Version | License |
| --- | --- | --- |
| `github.com/anacrolix/torrent` | `v1.61.1-0.20260823090931-70072f1a7460` | MPL-2.0 |
| `github.com/anacrolix/generics` | `v0.2.0` | MPL-2.0 |
| `github.com/egoist/mygo` | `v0.8.3` | MIT |
| `github.com/PuerkitoBio/goquery` | `v1.13.0` | BSD-3-Clause |
| `golang.org/x/sys` | `v0.49.0` | BSD-3-Clause |
| `golang.org/x/time` | `v0.16.0` | BSD-3-Clause |
| `modernc.org/sqlite` | `v1.61.1` | BSD-3-Clause |

## MPL-2.0 components

These modules are Mozilla Public License 2.0 Covered Software. Dipper uses them
unmodified. When distributing a binary, the corresponding source is the
upstream tree at the version below, and it can also be fetched with
`go mod download <module>@<version>`.

| Module | Version | Source |
| --- | --- | --- |
| `github.com/anacrolix/torrent` | `v1.61.1-0.20260823090931-70072f1a7460` | <https://github.com/anacrolix/torrent/tree/70072f1a7460> |
| `github.com/anacrolix/dht/v2` | `v2.23.1-0.20260525063928-ec3a9bd99456` | <https://github.com/anacrolix/dht/tree/ec3a9bd99456> |
| `github.com/anacrolix/generics` | `v0.2.0` | <https://github.com/anacrolix/generics/tree/v0.2.0> |
| `github.com/anacrolix/log` | `v0.17.1-0.20251118025802-918f1157b7bb` | <https://github.com/anacrolix/log/tree/918f1157b7bb> |
| `github.com/anacrolix/multiless` | `v0.4.0` | <https://github.com/anacrolix/multiless/tree/v0.4.0> |
| `github.com/anacrolix/sync` | `v0.5.5-0.20251119100342-d78dd1f686f1` | <https://github.com/anacrolix/sync/tree/d78dd1f686f1> |
| `github.com/anacrolix/upnp` | `v0.1.4` | <https://github.com/anacrolix/upnp/tree/v0.1.4> |
| `github.com/anacrolix/utp` | `v0.1.0` | <https://github.com/anacrolix/utp/tree/v0.1.0> |

If Dipper ever ships a modified MPL-2.0 file, the modified source will be
published in this repository and named in this section.

Two more MPL-2.0 modules, `github.com/anacrolix/mmsg` and
`github.com/go-llsqlite/adapter`, are linked only when CGO is enabled; see
[Modules linked only with CGO](#modules-linked-only-with-cgo).

## Other linked modules

The remaining modules in the release builds are listed by license. Their full
license texts ship with the modules and are available from the upstream
repositories or with `go mod download`.

### Apache-2.0

`github.com/anacrolix/btree@v0.1.1`,
`github.com/ebitengine/purego@v0.11.1`,
`github.com/go-logr/logr@v1.4.3`,
`github.com/go-logr/stdr@v1.2.2`,
`github.com/google/btree@v1.1.2`,
`github.com/mschoch/smat@v0.2.0`,
`github.com/RoaringBitmap/roaring/v2@v2.16.0`,
`go.opentelemetry.io/auto/sdk@v1.2.1`,
`go.opentelemetry.io/otel@v1.38.0`,
`go.opentelemetry.io/otel/metric@v1.38.0`,
`go.opentelemetry.io/otel/trace@v1.38.0`

### MIT

`github.com/alecthomas/atomic@v0.1.0-alpha2`,
`github.com/anacrolix/chansync@v0.8.0`,
`github.com/anacrolix/envpprof@v1.4.0`,
`github.com/anacrolix/missinggo@v1.3.0`,
`github.com/anacrolix/missinggo/perf@v1.0.0`,
`github.com/anacrolix/missinggo/v2@v2.11.0`,
`github.com/anacrolix/stm@v0.5.0`,
`github.com/benbjohnson/immutable@v0.4.1-0.20221220213129-8932b999621d`,
`github.com/cespare/xxhash@v1.1.0`,
`github.com/dustin/go-humanize@v1.1.0`,
`github.com/egoist/mygo@v0.8.3`,
`github.com/huandu/xstrings@v1.3.2`,
`github.com/klauspost/cpuid/v2@v2.2.3`,
`github.com/mattn/go-isatty@v0.0.24`,
`github.com/mr-tron/base58@v1.2.0`,
`github.com/multiformats/go-multihash@v0.2.3`,
`github.com/multiformats/go-varint@v0.0.6`,
`github.com/ncruces/go-strftime@v1.1.0`,
`github.com/pion/datachannel@v1.5.9`,
`github.com/pion/dtls/v3@v3.0.3`,
`github.com/pion/ice/v4@v4.0.2`,
`github.com/pion/interceptor@v0.1.40`,
`github.com/pion/logging@v0.2.3`,
`github.com/pion/mdns/v2@v2.0.7`,
`github.com/pion/randutil@v0.1.0`,
`github.com/pion/rtcp@v1.2.15`,
`github.com/pion/rtp@v1.8.18`,
`github.com/pion/sctp@v1.8.33`,
`github.com/pion/sdp/v3@v3.0.9`,
`github.com/pion/srtp/v3@v3.0.4`,
`github.com/pion/stun/v3@v3.0.0`,
`github.com/pion/transport/v3@v3.0.7`,
`github.com/pion/turn/v4@v4.0.0`,
`github.com/pion/webrtc/v4@v4.0.0`,
`github.com/protolambda/ctxlock@v0.1.0`,
`github.com/rs/dnscache@v0.0.0-20211102005908-e0241e321417`,
`github.com/tidwall/btree@v1.8.1`,
`go.etcd.io/bbolt@v1.3.6`,
`lukechampine.com/blake3@v1.1.6`

### BSD-3-Clause

`github.com/bahlo/generic-list-go@v0.2.0`,
`github.com/bits-and-blooms/bitset@v1.24.2`,
`github.com/bradfitz/iter@v0.0.0-20191230175014-e8f45d346db8`,
`github.com/edsrzf/mmap-go@v1.1.0`,
`github.com/go-text/typesetting@v0.3.5`,
`github.com/google/go-cmp@v0.7.0`,
`github.com/google/uuid@v1.6.0`,
`github.com/PuerkitoBio/goquery@v1.13.0`,
`github.com/remyoudompheng/bigfft@v0.0.0-20230129092748-24d4a6f8daec`,
`github.com/spaolacci/murmur3@v1.1.0`,
`github.com/wlynxg/anet@v0.0.3`,
`golang.org/x/crypto@v0.57.0`,
`golang.org/x/exp@v0.0.0-20251113190631-e25ba8c21ef6`,
`golang.org/x/image@v0.46.0`,
`golang.org/x/net@v0.59.0`,
`golang.org/x/sync@v0.24.0`,
`golang.org/x/sys@v0.49.0`,
`golang.org/x/text@v0.43.0`,
`golang.org/x/time@v0.16.0`,
`modernc.org/libc@v1.79.0`,
`modernc.org/mathutil@v1.7.1`,
`modernc.org/memory@v1.12.1`,
`modernc.org/sqlite@v1.61.1`

The `golang.org/x/*` modules also carry a `PATENTS` file granting additional
patent rights; it ships with each module.

### BSD-2-Clause

`github.com/andybalholm/cascadia@v1.3.4`,
`github.com/gorilla/websocket@v1.5.0`,
`github.com/pkg/errors@v0.9.1`

### ISC

`github.com/davecgh/go-spew@v1.1.1`

## Modules linked only with CGO

The `race` workflow builds with `CGO_ENABLED=1`, which links four modules that
release binaries do not contain:

| Module | Version | License |
| --- | --- | --- |
| `github.com/anacrolix/go-libutp` | `v1.5.0` | MIT |
| `github.com/anacrolix/mmsg` | `v1.0.1` | MPL-2.0 |
| `github.com/go-llsqlite/adapter` | `v0.0.0-20230927005056-7f5ce7f0c916` | MPL-2.0 |
| `github.com/go-llsqlite/crawshaw` | `v0.5.6-0.20250312230104-194977a03421` | ISC |

The pinned `go-llsqlite/adapter` snapshot has no license file; the upstream
repository is marked MPL-2.0. Its source is
<https://github.com/go-llsqlite/adapter>.

## Source-site sample data

The catalog snapshot `cmd/dipper/assets/catalog.json` and the cover images under
`cmd/dipper/assets/*.jpg` come from Mikan (`mikanani.me`). Their copyright
belongs to the respective rights holders. They are included only as sample data
for the interface and are not licensed under the MIT License. The screenshots
under `docs/assets/ui/` render this sample data and carry the same limitation.

The HTML and RSS captures under `internal/source/testdata/` are snapshots of
Mikan pages used only to test the parser. They are not original Dipper code and
are not licensed under the MIT License; their content belongs to Mikan and the
respective rights holders. Anyone redistributing the source should replace
these fixtures or confirm that their use is permitted.

Dipper does not host or distribute anime episodes. It reads a public catalog
and downloads torrents selected by the user.
