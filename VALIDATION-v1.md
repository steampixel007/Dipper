# Dipper 技术验证总览

- 验证日期：2026-10-20 至 2026-10-21。
- 目的：对已确定的 **MyGo Native UI + Go** 方案，验证三项关键风险是否成立。
- 状态：**三项验证均已实际运行并取得结果**（非纸面调研）。
- 环境：Windows amd64、Go 1.27.1、MyGo v0.5.5、anacrolix/torrent v1.61.0、CGO_ENABLED=0。

| 验证项 | 结论 |
|---|---|
| Windows 原生界面与桌面 E2E | **通过**：外部 UIA 可读取并操作控件；关闭进托盘、重启恢复均正确 |
| BT 引擎限速、停止、恢复与做种策略 | **通过**：共享限速、停止 payload、断点恢复 sha256 一致、策略持久化正确 |
| 蜜柑历史列表遍历与终止条件 | **已证实算法**：`take` 为绝对条数，返回行数≠take 即终止；长样本 1522 条 |

## 1. 原生界面与桌面 E2E（Windows）

已通过：

- 无窗口 `ui.NewTester` 测试 6 组全通过（点词建规则、勾选、暗色渲染、集数标签、目录安全、选择持久化）。
- 真实窗口启动后被外部 UI Automation 识别：`ControlType.Window`、`class=MyGoWindow`。
- 外部读取到完整控件树：**21 个 Text、33 个 Button、6 个 CheckBox**，中文词语名称正确。
- 外部 `Invoke` 词语按钮 → 规则行实时变化；外部 `Toggle` 复选框 → 摘要由 6 项变 5 项并落盘 `settings.json`。
- 关闭窗口后**进程存活**、窗口消失、日志记录 `window hidden`。
- 重启后读取保存状态：取消勾选的资源仍为未选，摘要恢复为 5 项。

未验证：托盘菜单交互、完全退出、macOS/Linux、动效回归、真实输入法。

## 2. BT 引擎（anacrolix/torrent v1.61.0）

已通过（`go test -count=1 ./...` 全 ok）：

| 项目 | 实测结果 |
|---|---|
| 3 任务共享 200 KiB/s 全局限速 | 各端 73.0/73.0/73.8 KiB/s，合计约 219.8 KiB/s；瞬时 159.7~240.6 KiB/s（burst 与采样波动） |
| `DisallowDataDownload` | 3 秒窗口 payload 增量 0 B；恢复后 +1,032,192 B |
| `DisallowDataUpload`（seeder） | 3 秒窗口 seeder payload 增量 0 B |
| `Client.Close` 停止传输 | leecher 关闭后 3 秒 seeder 增量 0 B、对端连接 2→0；seeder 关闭后 leecher2 增量 0 B |
| 部分下载后新 client 恢复 | 11/32 片 → 恢复至 24/32 片，落盘 sha256 与原始一致 |
| 做种策略持久化（虚拟时钟） | 跨重启累计不丢不重；达 3600 秒触发自动停止；手动停止标志跨重启保留；ratio 1.5 正确触发 |

过程中的两个重要修正（避免误判）：

1. `torrent.TestingConfig` 将 `MaxAllocPeerRequestDataPerConn` 设为 5 字节，必须还原为 1 MiB，否则传输永不开始；首轮超时由此造成，非引擎缺陷。
2. 策略测试原期望值写错（应为 15000 而非 11000），实现本身无重复计账问题。

未验证：真实公网 BT、多 seeder、真实 1 小时等待、OS 休眠/断网对计时的影响。

## 3. 蜜柑历史遍历

已证实（从站点 JS + 31 次限频请求取得）：

- 端点 `GET /Home/ExpandEpisodeTable`，参数仅 `bangumiId`、`subtitleGroupId`、`take`，**无 offset/page/cursor**。
- `take` 是**绝对条数**（非页大小）；初值来自页面 `data-take`（227 为 65），步进 +50。
- 终止条件：**返回行数 ≠ 请求的 take**（含 0 行），HTTP 始终 200。
- 长样本 227/161：实际 **1522 行 / 1522 唯一 episode id / 重复 0**；`take=1522` 返回 1522（继续），`take=1523` 返回 1522（终止）。
- 短样本 4063/370：3 行，首次请求即终止；页面无展开控件时即为全量。
- **RSS 硬上限 100 条**：长番历史 1522 条，RSS 只覆盖最新 100 条 ⇒ **不能用 RSS 做历史全量初始化**。
- 每行 episode id 即 BT infohash，可解析种子与磁力链接。

未证实：隐藏/删除条目是否返回、take 服务端硬上限、移动端端点、排序契约。

## 4. 对选型的意义

- **BT 引擎首选 `anacrolix/torrent` 已可用**，但需在产品中自行实现有效做种计时（接入系统休眠/网络事件）和停止意图持久化。
- **蜜柑接入必须走历史列表**（一次大 take 请求），RSS 仅用于后续增量；不能用 RSS 替代全量历史。
- **MyGo Native UI 的黑盒桌面 E2E 可行**：Windows 下可保留真实窗口自动化，不必只依赖进程内测试。
- 仍需补验：托盘菜单交互、完全退出、三端差异、动效与输入法回归。
