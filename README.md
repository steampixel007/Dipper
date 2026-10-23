# Dipper 0.1.0 MVP

基于 MyGo Native UI + Go 的原生追番桌面程序。延续已确认的 [交互设计](DESIGN-v1.md)，接入真实蜜柑目录、历史资源、RSS 与内置 BT 下载器。

## 启动

Windows 双击根目录的 **`start.cmd`**。便携程序为 `bin/dipper.exe`，不需要浏览器、Node 或外部下载器。

未签名的本地构建可能先显示 Windows SmartScreen 的“未知发布者”提示。选择继续运行后，单实例检查才会执行；正式分发应使用代码签名消除该提示。

首次启动读取当前季度。选择番剧 → 字幕组 → 点击标题词设置包含／排除条件 → 确认勾选 → 下载并追番。已有匹配资源默认全部勾选；取消勾选的记录跨重启保留。保存后每 15 分钟检查 RSS，失败后退避重试，也可立即检查。

“我的追番”可暂停、恢复、修改规则；从修改页可主动补下已有资源。修改规则仅影响未来未读条目，补下勾选不改变保存规则。暂停追番与暂停下载是两个独立操作。

下载页显示真实进度及总／单任务上传、下载速度，支持暂停、继续、重试、打开文件夹及停止做种。所有任务共享上传上限，默认 1 MiB/s（界面标为 MB/s）。完成后达到累计上传／下载 1:1 或有效做种累计 1 小时即停止。

关闭窗口收进托盘；在设置或托盘选择“完全退出”停止全部后台活动。托盘创建失败时关闭窗口直接退出，避免进程无法找回。

同一数据目录只运行一个主进程；再次双击程序会唤回已有窗口，最小化时恢复窗口。单实例检查先于数据库及下载引擎初始化，唤醒失败会显示错误，不会继续争用状态文件。

## 文件与状态

- `data/dipper.db`：SQLite，追番规则、已读／跳过条目、固定番剧目录、下载提交记录。
- `data/engine/`：BT 元数据、分片验证记录、暂停／停止意图与传输检查点。
- `data/preferences.json`：界面偏好。
- `data/covers/`：海报缓存；`data/app.log`：本机日志。
- `data/instance.lock` / `data/instance.json`：单实例所有权和本机窗口唤醒通道；异常退出后系统释放锁，下一次启动可恢复。
- 默认视频目录：系统用户“视频”目录下的 `Dipper` 文件夹（Windows 使用系统实际配置的视频路径，macOS 为 `~/Movies/Dipper`，Linux 遵循 `XDG_VIDEOS_DIR`），可在设置中更改。旧版默认的 `Downloads/Dipper` 会在重启后更新为新默认目录；已自定义路径与已订阅番剧的固定目录保留。

同一番剧跨字幕组沿用首次建立的目录；更改全局根目录只影响新番剧。同名不同番剧使用不同目录。每个 BT 资源有独立 hash 子目录，保留种子内原始文件名和目录结构。

## 开发

Go 版本及第三方依赖锁定在 `go.mod` / `go.sum`。Windows：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build.ps1 -Test -Render
```

加 `-CrossCompile` 构建 Linux amd64、macOS arm64／amd64 裸可执行文件。其他系统可在根目录运行 `CGO_ENABLED=0 go build ./cmd/dipper`；MyGo 的系统图形与托盘运行依赖仍需满足。

开发诊断参数：

```powershell
./bin/dipper.exe --data ./data
./bin/dipper.exe --probe-source
./bin/dipper.exe --render ./evidence/ui
```

`--probe-source` 只读取公开目录、详情、历史及 RSS，不创建下载。`--render` 使用开发测试夹具绘制同一套原生视图，不代表公网下载证据。可用 `--source URL` 对接本地来源测试服务，`--download-root PATH` 指定新数据的初始下载目录；`--quit-after 8s` 用于启动与退出验证。

## 验证与边界

具体结果见 [MVP 验收记录](MVP-v1.md)。已提供三系统 CI 配置，本机执行结果与未执行的目标系统验证分别记录。

本版为便携 MVP；未配置安装器、代码签名、自动更新、第三方插件安装市场及云同步。四类业务能力以内置 Go 模块与接口组合，尚未提供可安装插件的注册／版本协议。macOS/Linux 原生窗口、托盘、输入法及安装链路需要对应真机验收。Windows 未签名构建可能被 SmartScreen 拦截，签署发布证书后才能避免“未知发布者”提示。非 Windows 的网络可用性按活动网络接口判断，不等于已验证公网连通。
