# AI Passport 玩法社区 · Windows 7 版

给 **Windows 7 / 8 / 8.1** 用的原生桌面客户端：把 [AI Passport 玩法社区](https://ai-passport.folotoy.cn/plays/) 的全部玩法信息下载到电脑上，离线浏览，并通过 USB 把玩法刷入 AI Passport（ESP32-C3，固件在第一次刷入时下载并保存）；还能备份固件、恢复出厂固件、安装串口驱动。

A native client for Windows 7, 8 and 8.1: download every AI Passport play, browse offline, install plays over USB, back up or restore the firmware, and install the USB serial driver. No browser is needed.

Windows 10/11 和 macOS 请用 [ai-passport-desktop](https://github.com/FoloToy/ai-passport-desktop)。

## 下载和使用

在 [Releases](https://github.com/FoloToy/ai-passport-desktop-windows7/releases) 下载：

| 系统 | 文件 |
| --- | --- |
| 64 位 Windows 7/8/8.1 | `AI-Passport-Desktop-Windows7-<版本>-x64.zip` |
| 32 位 Windows 7/8/8.1 | `AI-Passport-Desktop-Windows7-<版本>-x86.zip` |

不知道是 32 位还是 64 位：右键"计算机"→ 属性 → 系统类型。

1. 把 ZIP 完整解压（不要在压缩包里直接运行），双击 `AIPassportPlays.exe`。不需要装浏览器、.NET 或其他运行库。
2. 第一次打开时选择玩法存储目录，点"开始更新"。
3. 插上 AI Passport。Windows 7/8 第一次要装串口驱动：驱动已经在程序里，程序发现设备缺驱动时会主动询问是否安装（见下一节）。
4. "玩法社区"页选择玩法 →"刷入设备"→ 选择设备 →"开始刷入"。

和旧的 Chrome 版 Windows 7 程序、以及桌面版用同一个设置目录（`%APPDATA%\FoloToy\AI Passport Plays`），已经下载的玩法库可以直接继续用。同一时间只能运行其中一个程序。

## 串口驱动

AI Passport 的 USB 口对电脑来说是一个设备、两个接口：

| 接口 | 硬件 ID | 用途 | Windows 10/11 | Windows 7/8 |
| --- | --- | --- | --- | --- |
| 串口（CDC） | `USB\VID_303A&PID_1001&MI_00` | **刷机必须用这个** | 自动装好系统自带的 `usbser.sys` | **需要装一次驱动** |
| JTAG 调试 | `USB\VID_303A&PID_1001&MI_02` | 只用于芯片调试，刷机不需要 | 自动 | 需要手动装 |

### 在程序里安装（推荐）

插上缺驱动的 AI Passport 后，程序会弹窗询问"现在安装吗？"（每次运行问一次，在哪个页面都会弹）；点"是"即开始安装。也可以随时在"设备工具"页 →"USB 串口驱动"→ 点"安装驱动"。刷机对话框和备份、恢复出厂找不到设备时，如果是缺驱动，也会直接说明并给出"安装驱动"按钮。

安装过程：UAC 窗口点"是"→ 如果提示"Windows 无法验证此驱动程序软件的发布者"，选"始终安装此驱动程序软件"→ 装好后重新插拔设备。

程序用的是 `drivers/ai-passport-usb-serial.inf`（源文件在 `internal/driver/`）：只给串口接口 `MI_00` 绑定 Windows 自带的 `usbser.sys`，用 `pnputil -i -a` 安装，不碰 JTAG 接口。INF 没有签名目录（.cat），所以 Windows 会提示一次发布者无法验证；`usbser.sys` 本身是微软签名的，64 位系统也能装。

### 用 Zadig 手动安装

程序装不上时：

1. 下载 Zadig：https://zadig.akeo.ie/
2. 用 USB 数据线插上 AI Passport（只插这一台）。
3. 右键 Zadig →"以管理员身份运行"。
4. 菜单 **Options → List All Devices** 打勾。
5. 下拉框里选 **USB JTAG/serial debug unit (Interface 0)**，确认 USB ID 是 `303A 1001 00`。**不要选 Interface 2，也不要选其他设备**（选错会让鼠标键盘等设备失灵）。
6. 右侧目标驱动切到 **USB Serial (CDC)**，点 **Install Driver** / **Replace Driver**。
7. 设备管理器"端口 (COM 和 LPT)"下应该出现一个 COM 口。

设备没有出现在 Zadig 列表里：按住设备的 BOOT 键再插 USB，让它停在下载模式后再操作。

### JTAG 驱动（可选）

只做芯片调试时才需要：`drivers/AI-Passport-Win7-JTAG-driver-Espressif-2021-07-15.zip`（Espressif 签名的 WinUSB 驱动，只覆盖 `MI_02`）。解压后在设备管理器里右键硬件 ID 末尾是 `MI_02` 的设备 → 更新驱动程序 → 浏览计算机 → 选择解压后的文件夹。

### 常见问题

| 现象 | 处理 |
| --- | --- |
| 装了 JTAG 驱动仍然没有 COM 口 | 正常，JTAG 驱动不管串口，按上面装串口驱动 |
| 64 位 Windows 7 提示驱动签名问题 | 先装 Windows 7 SP1 和 KB3033929（SHA-2 代码签名支持）、KB4474419 |
| 换 USB 口后又出现感叹号 | Windows 7 按端口记驱动，换口后再装一次 |
| 鼠标键盘失灵 | Zadig 里选错了设备。设备管理器里找到被替换的设备 → 卸载并删除驱动 → 重新插拔 |

## 架构

- **Go + Win32**：界面用 [lxn/walk](https://github.com/lxn/walk)（Windows 通用控件 6），没有浏览器、没有 WebView，一个 exe。
- **共用桌面版的代码**：玩法库、下载同步、刷机协议、恢复出厂、身份区保护都来自 [ai-passport-desktop](https://github.com/FoloToy/ai-passport-desktop) 的 `pkg/`（Go 模块依赖）。界面只调用 `pkg/core`，所以开始刷机、恢复、备份前的检查和桌面版完全一样。
- 用 [go-legacy-win7](https://github.com/thongtech/go-legacy-win7)（打了 Windows 7 补丁的 Go 1.26，版本固定并校验 SHA-256）编译，分 x64 和 x86。

| 目录 | 内容 |
| --- | --- |
| `main.go` | 启动、命令行参数（`-self-test -state-dir -origin`）、单实例 |
| `internal/ui/` | 窗口：玩法社区（`plays.go`）、刷入对话框（`flash.go`）、设备工具（`tools.go`）、设置（`settings.go`） |
| `internal/i18n/` | 中英文案；`web.json` 从桌面版 `frontend/app.js` 导出 |
| `internal/driver/` | Windows 7/8 串口驱动检测和安装 |
| `drivers/windows7/` | Espressif 的 JTAG 驱动 zip |
| `scripts/` | `package.sh`（打包）、`export-texts.sh`（同步文案） |

## 开发

需要 Go 1.26。界面代码只能在 Windows 上编译，可以在 Mac 或 Linux 上交叉编译：

```sh
export GOTOOLCHAIN=go1.26.0
go test ./...                                   # i18n、driver 等平台无关的部分
GOOS=windows GOARCH=amd64 go vet ./...          # 检查 Windows 代码
ALLOW_DIRTY=1 scripts/package.sh                # 用 go-legacy-win7 打 x64 和 x86 两个 ZIP 到 dist/
```

用 AI Agent 开发时先读 [AGENTS.md](AGENTS.md)。发版步骤见 [docs/RELEASE.md](docs/RELEASE.md)。

## 许可证

源代码以 [MIT 许可证](LICENSE) 发布。第三方组件、内置的 FoloToy 出厂固件和 Espressif 驱动不在 MIT 范围内，见 [THIRD_PARTY.md](THIRD_PARTY.md)。
