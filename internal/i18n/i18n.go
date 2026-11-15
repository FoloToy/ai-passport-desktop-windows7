// Package i18n holds the Chinese and English text of the Windows 7 client.
//
// web.json is exported from the desktop app's frontend/app.js (I18N.zh / I18N.en,
// string values only) so both clients use the same wording for stages, errors and
// hints; scripts/export-texts.sh refreshes it. native holds the text only this
// client needs; formatted strings are functions below.
package i18n

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/FoloToy/ai-passport-desktop/pkg/library"
)

//go:embed web.json
var webJSON []byte

var web struct {
	Zh map[string]string `json:"zh"`
	En map[string]string `json:"en"`
}

func init() {
	if err := json.Unmarshal(webJSON, &web); err != nil {
		panic("i18n: " + err.Error())
	}
}

// native is the text this client adds; each entry is {zh, en}.
var native = map[string][2]string{
	"app_title":            {"AI Passport 玩法社区", "AI Passport App Library"},
	"edition":              {"Windows 7 版", "Windows 7 edition"},
	"col_name":             {"玩法", "App"},
	"col_category":         {"分类", "Category"},
	"col_author":           {"作者", "Author"},
	"col_downloads":        {"下载量", "Downloads"},
	"col_updated":          {"发布时间", "Published"},
	"col_status":           {"固件", "Firmware"},
	"all_categories":       {"全部分类", "All categories"},
	"only_ready":           {"只看已下载固件的", "Downloaded only"},
	"select_play":          {"在左边选择一个玩法。", "Pick an app on the left."},
	"fetch_btn":            {"下载固件", "Download firmware"},
	"fetching":             {"正在下载固件…", "Downloading firmware…"},
	"fetch_done":           {"固件已下载，可以刷入了。", "Firmware downloaded. You can install it now."},
	"port_label":           {"设备", "Device"},
	"choose_port":          {"请选择设备。", "Choose a device."},
	"close":                {"关闭", "Close"},
	"log":                  {"日志", "Log"},
	"device_busy":          {"正在操作设备", "Device busy"},
	"busy_close":           {"请等待刷机、备份或恢复出厂完成后再关闭，否则设备可能无法启动。", "Wait for the install, backup or factory restore to finish before closing; otherwise the device may not start."},
	"already_running":      {"程序已经在运行了。", "The app is already running."},
	"lang_restart":         {"语言会在下次打开程序时生效。", "The language changes the next time you open the app."},
	"lang_zh":              {"中文", "中文"},
	"lang_en":              {"English", "English"},
	"choose_library":       {"选择玩法存储目录", "Choose the library folder"},
	"choose_backup":        {"选择备份目录", "Choose the backup folder"},
	"no_library":           {"还没有玩法。请先在下面选择存储目录，然后点“开始更新”。", "No apps yet. Choose a library folder below, then click “Start download”."},
	"library_empty":        {"没有符合条件的玩法。", "No apps match."},
	"sync_phase":           {"更新玩法库：", "Updating: "},
	"driver_title":         {"USB 串口驱动", "USB serial driver"},
	"driver_ok":            {"没有发现缺少驱动的 AI Passport。如果插上设备后“设备管理器”里出现黄色感叹号，再点“安装驱动”。", "No AI Passport without a driver was found. If the device shows a yellow exclamation mark in Device Manager after you plug it in, click “Install driver”."},
	"driver_missing":       {"这台电脑还没有 AI Passport 的串口驱动（Windows 7/8 需要安装一次）。点“安装驱动”，在弹出的窗口里点“是”；如果提示“无法验证发布者”，选“始终安装此驱动程序软件”。装好后重新插拔设备。", "This PC has no driver for the AI Passport serial port (Windows 7/8 need it installed once). Click “Install driver” and answer “Yes”; if Windows cannot verify the publisher, choose “Install this driver software anyway”. Reconnect the device afterwards."},
	"driver_ask":           {"插着的 AI Passport 还没有串口驱动，Windows 7/8 需要安装一次才能刷机和备份。现在安装吗？\r\n\r\n点“是”后 Windows 会请求管理员权限，请点“是”；如果提示“无法验证发布者”，选“始终安装此驱动程序软件”。装好后重新插拔设备。", "Your AI Passport is plugged in, but this PC has no driver for its USB serial port. Windows 7 and 8 need it installed once before you can install apps or back up. Install it now?\r\n\r\nWindows will ask for administrator permission; click “Yes”. If Windows can't verify the publisher, choose “Install this driver software anyway”. Reconnect the device afterwards."},
	"driver_missing_short": {"找不到设备，因为这台电脑还没有 AI Passport 的串口驱动。点“安装驱动”，装好后重新插拔设备。", "Can't find the device because this PC has no driver for the AI Passport serial port. Click “Install driver”, then reconnect the device."},
	"driver_install":       {"安装驱动", "Install driver"},
	"driver_installing":    {"正在安装驱动…", "Installing the driver…"},
	"driver_done":          {"驱动已安装，请重新插拔设备。", "Driver installed. Reconnect the device."},
	"err_driver":           {"驱动没有装好。可以按 README 的说明用 Zadig 手动安装。", "The driver was not installed. See the README to install it with Zadig."},
	"website":              {"玩法社区网站", "Community website"},
	"factory_fw":           {"出厂固件", "Factory firmware"},
	"default_folder":       {"默认", "default"},
	"stage_preparing":      {"正在准备", "Preparing"},
	"stage_waiting":        {"正在等待设备", "Waiting for the device"},
	"stage_verifying":      {"正在校验", "Verifying"},
	"stage_resetting":      {"正在重启设备", "Restarting the device"},
	"stage_cardid":         {"正在检查身份区", "Checking the identity area"},
	"stage_backup":         {"正在备份", "Backing up"},
	"backup_go":            {"开始备份", "Start backup"},
	"restore_go_q":         {"确定要恢复出厂固件吗？", "Restore the factory firmware?"},
	"tab_details":          {"详情", "Details"},
	"official":             {"官方", "Official"},
	"updated_at":           {"更新于", "Updated"},
	"published_at":         {"发布于", "Published"},
	"never_synced":         {"玩法库还没有下载。", "The library has not been downloaded yet."},
	"plays_summary":        {"共 %d 款玩法，%d 款可刷入", "%d apps, %d installable"},
	"synced_at":            {"上次更新：%s", "Last update: %s"},
}

// Lang is "zh" or "en".
type Lang string

// Pick resolves the saved setting ("zh", "en" or "" for the system language).
func Pick(saved, system string) Lang {
	switch {
	case saved == "en":
		return "en"
	case saved == "zh":
		return "zh"
	case strings.HasPrefix(system, "en"):
		return "en"
	}
	return "zh"
}

var current Lang = "zh"

// Set chooses the language for T. Call it once before the UI is built.
func Set(l Lang) { current = l }

// Current returns the language in use.
func Current() Lang { return current }

// T returns the text for key, falling back to Chinese and then to the key itself.
func T(key string) string {
	if n, ok := native[key]; ok {
		if current == "en" && n[1] != "" {
			return n[1]
		}
		return n[0]
	}
	if current == "en" {
		if v, ok := web.En[key]; ok {
			return v
		}
	}
	if v, ok := web.Zh[key]; ok {
		return v
	}
	return key
}

// Tf formats a native text that has %-verbs.
func Tf(key string, args ...any) string { return fmt.Sprintf(T(key), args...) }

// Has reports whether key has a translation.
func Has(key string) bool {
	if _, ok := native[key]; ok {
		return true
	}
	_, ok := web.Zh[key]
	return ok
}

// ErrText translates an error code (core.Code, flasher and syncer codes); fallback
// is the raw message shown when the code is unknown.
func ErrText(code, fallback string) string {
	if code != "" && Has("err_"+code) {
		return T("err_" + code)
	}
	if fallback != "" {
		return fallback
	}
	return T("err_other")
}

// Text returns the text in the current language, falling back to the other one.
func Text(t library.Text) string {
	if current == "en" && t.En != "" {
		return t.En
	}
	if t.Zh != "" {
		return t.Zh
	}
	return t.En
}

// The formatted texts of frontend/app.js.

func Results(n int) string {
	if current == "en" {
		return fmt.Sprintf("%d apps", n)
	}
	return fmt.Sprintf("共 %d 款玩法", n)
}

func Throttled(wait string) string {
	if current == "en" {
		return "The firmware server is rate limiting; resuming in " + wait
	}
	return "固件服务器限流中，" + wait + " 后自动继续"
}

func FirmwareProgress(a, b int) string {
	if current == "en" {
		return fmt.Sprintf("Firmware %d / %d", a, b)
	}
	return fmt.Sprintf("固件 %d / %d", a, b)
}

func FetchWait(wait string) string {
	if current == "en" {
		return "The firmware server is busy. Try again in " + wait + "."
	}
	return "固件服务器繁忙，请在 " + wait + " 后重试。"
}

func SyncCounts(added, updated, removed int) string {
	if current == "en" {
		return fmt.Sprintf("%d new, %d updated, %d removed", added, updated, removed)
	}
	return fmt.Sprintf("新增 %d，更新 %d，下架 %d", added, updated, removed)
}

func BannerUpdate(n int) string {
	if current == "en" {
		return fmt.Sprintf("%d new or updated apps are available.", n)
	}
	return fmt.Sprintf("玩法社区有 %d 个新玩法或更新。", n)
}

func Waited(s, max int) string {
	if current == "en" {
		return fmt.Sprintf("Waited %d s (up to %d s)", s, max)
	}
	return fmt.Sprintf("已等待 %d 秒（最多 %d 秒）", s, max)
}

func Code43(port string) string {
	if current == "en" {
		return "Windows reports a USB error on " + port + ". Unplug and replug the device once."
	}
	return "Windows 报告设备 USB 异常（" + port + "），请拔插一次设备。"
}

// Bytes formats a size as KB / MB / GB.
func Bytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}
