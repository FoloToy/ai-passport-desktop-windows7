# AGENTS.md

写给在这个仓库里工作的 AI Agent（Claude Code、Codex、Cursor 等）和新加入的开发者。先读这一页，再看 `README.md`。

## 项目是什么

**AI Passport · Windows 7 版**（0.2 及以前叫「AI Passport 玩法社区」）：给 Windows 7 / 8 / 8.1 用的原生 Win32 客户端。功能和桌面版一样：下载并离线浏览玩法、刷入玩法、备份固件、恢复出厂，另外能安装串口驱动。

- 仓库：`FoloToy/ai-passport-desktop-windows7`（公开，MIT），默认分支 `main`，PR 也提到 `main`。
- 姊妹仓库：[FoloToy/ai-passport-desktop](https://github.com/FoloToy/ai-passport-desktop)（macOS、Windows 10/11）。**刷机、恢复出厂、同步、玩法库的代码都在那边的 `pkg/`**，这里通过 Go 模块引用，不要复制过来。
- 只支持 ESP32-C3，一次一台设备。界面中英双语。

## 技术栈

| 部分 | 实现 |
| --- | --- |
| 语言 | Go 1.26；发布版用 **go-legacy-win7 v1.26.11-1** 编译（`scripts/package.sh` 里固定版本和 SHA-256） |
| 界面 | `github.com/lxn/walk`（declarative），Win32 通用控件 6；manifest 在 `rsrc_windows_*.syso` 里 |
| 共用逻辑 | `github.com/FoloToy/ai-passport-desktop/pkg/core` 及以下（`esp device factory firmware flasher library settings syncer instance`） |
| 图片 | 封面是 WebP/JPEG/PNG，用 `golang.org/x/image/webp` 和标准库解码 |
| 驱动 | `internal/driver`：注册表 + cfgmgr32 检测，`pnputil` 通过 UAC 安装内置 INF |

## 目录地图

```
main.go             启动、-self-test、单实例（已在运行时把旧窗口提到前台）
main_other.go       非 Windows 时的占位 main，让 go test ./... 在 Mac/Linux 上能跑
internal/ui/        app.go（主窗口、轮询、工具函数、AIPP_CAPTURE 截图）
                    plays.go（玩法列表、筛选、详情、更新玩法库）
                    flash.go（刷入对话框）tools.go（备份、恢复出厂、驱动）settings.go
internal/i18n/      i18n.go（本客户端的文案 + 带参数的文案）web.json（从桌面版导出）
internal/driver/    串口驱动检测与安装、INF（CRLF）
assets/             图标
drivers/windows7/   Espressif JTAG 驱动 zip
scripts/            package.sh（打包）export-texts.sh（同步文案）
```

## 常用命令

```sh
export GOTOOLCHAIN=go1.26.0
go test ./...                                    # 平台无关部分（i18n、driver INF）
GOOS=windows GOARCH=amd64 go vet ./...
GOOS=windows GOARCH=386 go vet ./...
ALLOW_DIRTY=1 scripts/package.sh                 # go-legacy-win7：测试 + vet + 编译 x64/x86 + ZIP
scripts/export-texts.sh ../ai-passport-desktop   # 桌面版文案改了之后同步 web.json
```

`gofmt` 只对项目文件跑（`gofmt -l *.go internal assets`），不要 `gofmt .`：`.runtime/` 里有整个 go-legacy-win7 源码。

### 在 Windows 上看界面

界面无法在 Mac 上运行。在 Windows 机器上设环境变量 `AIPP_CAPTURE=<文件夹>` 再启动，程序会把四个标签页（设备 / 玩法 / 工具 / 我的）、显示了展会批量下载的“我的”页、登录、我的备份、第一个玩法的评论，以及第一个已下载玩法的刷入对话框（不会开始刷入）保存成 PNG 后自动退出。通过 SSH 运行时程序在非交互会话里，控件不绘制，需要用计划任务（`schtasks /IT`）在已登录用户的会话里启动。

### 升级共用代码

```sh
go get github.com/FoloToy/ai-passport-desktop@<commit 或 tag> && go mod tidy
```

升级后跑一遍 `go test ./...`、两个架构的 `vet` 和 `scripts/package.sh`。桌面版改了 `pkg/` 的导出 API 时这里要跟着改。

## 必须遵守的规则

### 设备安全

- **不要在这个仓库里实现任何刷机、擦除、写 Flash 的逻辑**。所有设备任务都通过 `core.Core` 的 `StartFlash`、`StartRestore`、`StartBackup` 开始，安全规则（不写身份区 cardid、不整片擦除、写入后不能取消、安全启动/加密时拒绝写入、只接受 8 MB ESP32-C3）在桌面版的 `pkg/` 里实现和测试。
- 设备任务进行中不能关闭主窗口或刷入对话框（`Closing` 事件里拦截），不要去掉。
- **不要在没有用户明确同意的情况下往真机写东西**。备份文件含设备序列号和密钥，不要提交或上传。

### 界面

- 所有用户可见文字都要中英两份：本客户端特有的写在 `internal/i18n/i18n.go` 的 `native`，和桌面版相同的文案从 `web.json` 取（不要手改 `web.json`，用 `scripts/export-texts.sh`）。`go test ./internal/i18n/` 会检查两种语言都齐。
- 错误码用 `i18n.ErrText(core.Code(err), err.Error())` 翻译，和桌面版的 `err_<code>` 一致。
- 标签文字里的 `&` 会被 Windows 当成快捷键标记，显示玩法名称等外部文本时用 `plain()`。
- 多行文本控件要用 CRLF（`crlf()`）。
- 只能用 Windows 7 有的 API。新加 Win32 调用前确认 Windows 7 SP1 支持，必要时用 `LazyProc.Find()` 判断。
- UI 只能在主线程改；后台 goroutine 用 `mw.Synchronize`。

### 文件格式

- `*.inf` 是 CRLF（`.gitattributes`，`internal/driver` 的测试会检查）。
- Go 代码提交前 `gofmt`。

### 社区接口

- 固件下载接口按 IP 限流（20–30 次后返回 429，封 30 分钟以上）。测试时不要反复全量下载。
- 所以“更新玩法库”默认只下载玩法信息和图片，固件在第一次刷入时按需下载（详情里的“下载固件”）并保存。没有“下载全部固件”。隐藏功能“展会批量下载”：在“我的”页关于里的版本号上 3 秒内连点 5 次显示，只下载玩法首页第一屏的 16 个玩法（`firstScreen` → `core.StartBatch`）。
- 账号和统计与桌面版、安卓一致：登录只有验证码登录（手机号带国家区号，发 E.164；或邮箱），首次登录自动注册；请求都经过桌面版的 `pkg/account`（同一个 `session.json`、白名单和 CSRF）。选中玩法 0.8 秒后记一次浏览（`POST /api/engagement/{kind}/{slug}/view`），刷入玩法成功且已登录时记安装历史。
- 界面和安卓一致：设备 / 玩法 / 工具 / 我的四个标签，设置在“我的”右半边。需要蓝牙或 NFC 的功能显示为“需要蓝牙”，不能点。

### Git

- Conventional Commits（`feat:`、`fix:`、`docs:`、`chore:`…）。只有用户要求时才提交、推送或发版。
- 仓库是公开的，不要提交 `dist/`、`.runtime/`、备份文件、本机路径或个人信息。

## 还没验证的部分

- **没有在 Windows 7 / 8 真机上运行过**。在 Windows 11 上跑过：自检通过，四个标签页、登录、我的备份、评论和刷入对话框显示正常（读取了 1295 款玩法的真实玩法库）。登录、点赞、收藏、评论、消息等账号功能没有用真实账号在这个客户端里试过。
- 驱动安装、刷入玩法、备份、恢复出厂都没有在这个客户端里接真机跑过（共用代码在桌面版里验证过读和备份）。
- 驱动 INF 没有签名目录（.cat）。
