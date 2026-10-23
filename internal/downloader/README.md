# 内置 BT 适配器

包路径 `dipper.local/dipper/internal/downloader`，Go 1.27.1。

直接依赖：`github.com/anacrolix/torrent v1.61.0`、`golang.org/x/time v0.14.0`。

## core/UI 接入

```go
engine, err := downloader.New(downloader.Config{
    DataDir: stateDirectory,
    UploadLimit: settings.UploadLimitBytes,
})
err = engine.Ensure(downloader.Task{
    ID: task.Hash,
    Magnet: task.Resource.Magnet,
    TorrentURL: task.Resource.TorrentURL,
    Folder: task.Directory,
    Context: task.Context,
})
```

`Pause(id) error`、`Resume(id) error`、`StopSeeding(id) error`、`SetUploadLimit(bytes) error`、`Close() error`。`Snapshot() []Snapshot` 返回按 ID 排序的独立快照，速度单位 B/s，Progress 范围 0–1，SeedingSeconds 单位秒。状态为 metadata/downloading/seeding/paused/stopped/completed/error；completed 表示策略自动停止，Complete 表示文件已经校验完整。

`Task.ID` 为 40 位十六进制时，提交前必须与 magnet/.torrent 实际 v1 info-hash 相等。支持普通不透明任务 ID 作为包内/独立调用方式。当前支持 v1 与含 v1 hash 的 hybrid torrent，纯 v2 明确返回错误。相同 ID 和 Folder 的重复 Ensure 不改变暂停或停止意图，允许 tracker、URL 参数变化；新传入的 magnet 仍会验证身份。重复提交不会为了检查一个变化的 URL 再次发 HTTP 请求。

同时提供 magnet 和 TorrentURL 时，优先 HTTP 获取元数据，核对两者 hash 后合并 tracker 和显式 peer。获取失败时回退 magnet；身份不匹配直接失败，Context 取消不会回退。元数据 HTTP 请求有 30 秒超时、16 MiB 上限。Context 只控制任务接纳和 HTTP 请求，已接纳任务由引擎生命周期控制。完全退出先关闭 core，再 Close 引擎；Close 会取消尚未完成的 Ensure HTTP 请求。

core 传入每资源独立 Folder；引擎保留 torrent 内根名称、文件名和相对结构。通过 os.Root 约束所有内容 IO，拒绝路径逃逸、Windows 设备名、路径冲突及任务目录重叠。引擎状态目录与内容目录必须互不包含。

## 持久化与做种

DataDir 保存原子替换的 tasks.json、以任务 ID 的 SHA-256 命名的 .torrent、pieces 下的分片完成数据库。每 500ms 采样并持久化；暂停、停止和正常退出额外保存。崩溃可能丢失最近一个采样窗口的 payload/时间统计，数据恢复会重新校验文件分片。暂停、手停和自动停止跨重启保留。正常 Close 不改用户意图，恢复运行中的任务不会累计退出时间或重复计账。

下载期间即可上传；单 client 的一个限速器约束全部任务。Config.UploadLimit=0 使用默认 1 MiB/s；负值不限速。运行期 SetUploadLimit(0) 将上传速率预算设为零，负值不限速。令牌桶 burst 为 32 KiB，预算调整前已放行或预留的块可能继续发出；暂停/停止/Close 使用 Drop 断开连接以停止该任务全部 payload。

累计上传 payload 达到累计下载 payload，或完成后有效做种累计达到 3600 秒，即停止做种，保留文件。预先存在的完整文件在下载累计为零时，以内容大小作为分享目标，避免 0/0 立即停止。有效时长要求采样区间两端均完整、运行、允许做种且网络可用；不要求存在下载者。采样间隔超过 3 秒时丢弃整个区间，避免把失去观测的时间算入。Resume 清除暂停和手停，不清空累计统计，也不绕过已达到的自动停止条件。

Windows 自动使用 [GetNetworkConnectivityHint](https://learn.microsoft.com/en-us/windows/win32/api/netioapi/nf-netioapi-getnetworkconnectivityhint) 的互联网连接提示及 [QueryUnbiasedInterruptTime](https://learn.microsoft.com/en-us/windows/win32/api/realtimeapiset/nf-realtimeapiset-queryunbiasedinterrupttime) 的排除休眠计时。Linux/macOS 检查 Up、Running、非回环网卡及可用单播地址，仅代表接口可用，不能保证公网可达。

主程序应把 MyGo Power 的 suspend/resume 事件传给 `SetSuspended(bool)`；可把权威系统网络事件传给 `SetNetworkAvailable(bool)` 覆盖自动检测。状态切换重置计时资格，这两个方法不改变用户的持久意图。

## 验证

`go test -mod=readonly -count=1 -timeout=180s ./internal/downloader`

所有引擎测试绑定 127.0.0.1，关闭 DHT、tracker、PEX、uTP、webseed 和 UPnP；种子由测试自主生成，通过独立内存 seed 的真实 BT 连接传给引擎，验证多文件、字幕和空文件的 SHA-256。覆盖暂停/恢复、无 peer 的元数据恢复、跨重启计数和手停、真实 1:1 自动停止、共享上传预算、退出后零 payload、取消、身份冲突及路径约束。1 小时策略用虚拟时钟验证。

pipeline_test.go 额外把真实 core.SQLite → adapter → engine 串联：历史资源选择 → 真实下载 → RSS 重复去重 → core/engine 重启 → 被取消资源仍为 skipped。尚未验证真实公网 BT、机器实际休眠及非 Windows 实机行为。
