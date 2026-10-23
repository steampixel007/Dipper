# Notes for coding agents

这个文件只做一件事：告诉你干活之前该翻哪一份文档。规则本身不写在这里，
写在被指向的文件里。别往这个文件里加规则。

## 改代码之前

读 [LESSONS.md](LESSONS.md)：最近合进来的改动出了什么错，以及哪条规矩能提前拦住它。

## 写代码、审代码

对照 [docs/code-standards.md](docs/code-standards.md)。改动的验收标准在那里，
包括那条「测试必须在不带修复时失败」和怎么证明它。

## 按子系统翻文档

| 要改的东西 | 先读 |
| --- | --- |
| `internal/core` 的集成、状态库、规则、下载根目录 | [internal/core/README.md](internal/core/README.md) |
| `internal/downloader` 的 BT 引擎、分片校验、传输策略、共享限速 | [internal/downloader/README.md](internal/downloader/README.md) |
| `internal/source` 的抓取、HTML 解析、标题匹配 | 直接读 `internal/source/` 下的源码；`testdata/` 里存着真实页面样本 |
| `internal/instance` 的单实例锁与窗口唤醒 | 直接读 `internal/instance/` 下的源码，注意 `lock_windows.go` 与 `lock_other.go` 的平台差异 |
| `cmd/dipper` 的界面与交互 | [DESIGN-v1.md](DESIGN-v1.md) 定交互，`cmd/dipper/view.go` 是渲染主体 |

## 平台差异

`*_windows.go` 与 `*_other.go` 是同一件事的两套实现，改一边必须看另一边。
构建要过三个平台，见 `.github/workflows/check.yml`。

## 抓取相关的测试

`internal/source` 会碰真网络。写这类测试时用 `testdata/` 里的页面样本，
不要请求线上站点；确需联网的用例加 `testing.Short()` 跳过，并在 PR 里说明没跑。

## 提交之前

跑 `scripts/build.ps1 -Test`，它会做 `go test -count=1 -timeout 5m ./...` 和 `go vet ./...`。
新写的测试要证明它抓得住 bug，用 [cmd/redcheck](cmd/redcheck) 判定，
不要凭「测试变红了」就认定它算数。
