//go:build windows

// Package ui is the native Win32 window of the Windows 7 client, built with
// lxn/walk (common controls 6, no browser and no WebView). All app logic goes
// through pkg/core of ai-passport-desktop, so the checks before every device job
// are the same as in the desktop app.
package ui

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/FoloToy/ai-passport-desktop/pkg/core"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
	_ "golang.org/x/image/webp"

	"github.com/FoloToy/ai-passport-desktop-windows7/assets"
	"github.com/FoloToy/ai-passport-desktop-windows7/internal/i18n"
)

var t = i18n.T

// Title is the main window title; a second instance uses it to find the first.
func Title() string { return t("app_title") + "（" + t("edition") + "）" }

type app struct {
	c    *core.Core
	mw   *walk.MainWindow
	tabs *walk.TabWidget

	plays    *playsPage
	tools    *toolsPage
	settings *settingsPage
	status   *walk.StatusBarItem
	version  *walk.StatusBarItem

	// flashDlg is non-nil while the install dialog is open.
	flashDlg *flashDialog
	done     chan struct{}

	// driverMissing is true while a plugged-in AI Passport has no serial driver
	// (Windows 7/8). driverAsked keeps the install prompt to once per run.
	driverMissing bool
	driverAsked   bool
}

// form is the window message boxes belong to: the install dialog when it is open.
func (a *app) form() walk.Form {
	if a.flashDlg != nil && a.flashDlg.dlg != nil {
		return a.flashDlg.dlg
	}
	return a.mw
}

// Run shows the main window and returns when it is closed.
func Run(c *core.Core) error {
	a := &app{c: c, plays: &playsPage{}, tools: &toolsPage{}, settings: &settingsPage{}, done: make(chan struct{})}
	a.plays.a, a.tools.a, a.settings.a = a, a, a

	err := MainWindow{
		AssignTo: &a.mw,
		Title:    Title(),
		Font:     uiFont(9, false),
		Size:     Size{Width: 1120, Height: 720},
		MinSize:  Size{Width: 880, Height: 580},
		Layout:   VBox{Margins: Margins{Left: 6, Top: 6, Right: 6, Bottom: 0}},
		Children: []Widget{
			TabWidget{
				AssignTo: &a.tabs,
				Pages: []TabPage{
					a.plays.page(),
					a.tools.page(),
					a.settings.page(),
				},
			},
		},
		StatusBarItems: []StatusBarItem{
			{AssignTo: &a.status, Width: 640},
			{AssignTo: &a.version, Width: 220},
		},
	}.Create()
	if err != nil {
		return err
	}
	if img, _, err := image.Decode(bytes.NewReader(assets.Icon)); err == nil {
		if icon, err := walk.NewIconFromImageForDPI(img, a.mw.DPI()); err == nil {
			_ = a.mw.SetIcon(icon)
		}
	}
	a.version.SetText(t("version") + " " + c.Version)

	a.mw.Closing().Attach(func(canceled *bool, _ walk.CloseReason) {
		if c.Flash.Active() {
			*canceled = true
			walk.MsgBox(a.mw, t("device_busy"), t("busy_close"), walk.MsgBoxIconWarning)
		}
	})
	c.OnIndexChange(func() { a.mw.Synchronize(a.plays.reload) })

	a.plays.init()
	a.tools.init()
	a.settings.init()
	a.refresh()
	go a.poll()
	if dir := os.Getenv("AIPP_CAPTURE"); dir != "" {
		go a.capture(dir)
	}

	a.mw.Run()
	close(a.done)
	c.Sync.Cancel()
	return nil
}

// poll refreshes the window from the sync and device state a few times a second.
func (a *app) poll() {
	tick := time.NewTicker(300 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-a.done:
			return
		case <-tick.C:
			a.mw.Synchronize(a.refresh)
		}
	}
}

func (a *app) refresh() {
	a.plays.refresh()
	a.tools.refresh()
	a.settings.refresh()
	if a.flashDlg != nil {
		a.flashDlg.refresh()
	}
	info := a.c.LibraryInfo()
	switch {
	case !info.HasIndex:
		setText(a.status, t("never_synced"))
	default:
		s := i18n.Tf("plays_summary", info.Plays, info.Ready)
		if !info.SyncedAt.IsZero() {
			s += "    " + i18n.Tf("synced_at", info.SyncedAt.Local().Format("2006-01-02 15:04"))
		}
		setText(a.status, s)
	}
}

func (a *app) showError(err error) {
	if err == nil {
		return
	}
	walk.MsgBox(a.mw, t("app_title"), i18n.ErrText(core.Code(err), err.Error()), walk.MsgBoxIconError)
}

// chooseFolder shows the Windows folder picker.
func chooseFolder(owner walk.Form, title, current string) (string, bool) {
	dlg := &walk.FileDialog{Title: title, FilePath: current, InitialDirPath: current}
	ok, err := dlg.ShowBrowseFolder(owner)
	if err != nil || !ok || dlg.FilePath == "" {
		return "", false
	}
	return dlg.FilePath, true
}

// reveal opens a folder in Explorer.
func reveal(dir string) {
	if dir == "" {
		return
	}
	_ = os.MkdirAll(dir, 0o755)
	_ = exec.Command("explorer", dir).Start()
}

// openURL opens a web page in the default browser.
func openURL(url string) {
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return
	}
	verb, _ := syscall.UTF16PtrFromString("open")
	file, _ := syscall.UTF16PtrFromString(url)
	win.ShellExecute(0, verb, file, nil, nil, win.SW_SHOWNORMAL)
}

// Activate brings the window of an already running instance to the front. It
// reports false when no such window exists.
func Activate() bool {
	title, _ := syscall.UTF16PtrFromString(Title())
	h := win.FindWindow(nil, title)
	if h == 0 {
		return false
	}
	if win.IsIconic(h) {
		win.ShowWindow(h, win.SW_RESTORE)
	}
	win.SetForegroundWindow(h)
	return true
}

// Notice shows a message box without a main window.
func Notice(msg string) { walk.MsgBox(nil, t("app_title"), msg, walk.MsgBoxIconInformation) }

// crlf turns text into what Windows edit controls display as separate lines.
func crlf(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}

// texter is any widget with Text/SetText; setText skips identical text so the
// 300 ms refresh does not make controls flicker.
type texter interface {
	Text() string
	SetText(string) error
}

func setText(w texter, s string) {
	if w != nil && w.Text() != s {
		_ = w.SetText(s)
	}
}

func setVisible(w walk.Widget, v bool) {
	if w != nil && w.Visible() != v {
		w.SetVisible(v)
	}
}

func setEnabled(w walk.Widget, v bool) {
	if w != nil && w.Enabled() != v {
		w.SetEnabled(v)
	}
}

func setProgress(p *walk.ProgressBar, v int) {
	if p != nil && p.Value() != v {
		p.SetValue(v)
	}
}

// loadImage decodes a library image (WebP, JPEG, PNG or GIF) into a bitmap.
func loadImage(path string, dpi int) *walk.Bitmap {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil
	}
	bmp, err := walk.NewBitmapFromImageForDPI(img, dpi)
	if err != nil {
		return nil
	}
	return bmp
}

// uiFont is the window font: Microsoft YaHei for Chinese and Segoe UI for
// English, both shipped with every Windows 7 language edition.
func uiFont(points int, bold bool) Font {
	family := "Segoe UI"
	if i18n.Current() == "zh" {
		family = "Microsoft YaHei"
	}
	return Font{Family: family, PointSize: points, Bold: bold}
}

// sentences joins sentences: Chinese needs no space between them, English does.
func sentences(s ...string) string {
	if i18n.Current() == "en" {
		return strings.Join(s, " ")
	}
	return strings.Join(s, "")
}

// plain keeps "&" visible in labels, where Windows reads it as a mnemonic.
func plain(s string) string { return strings.ReplaceAll(s, "&", "&&") }

var red = walk.RGB(192, 32, 32)
var grey = walk.RGB(96, 96, 96)
var green = walk.RGB(16, 128, 48)

// capture is a development aid (AIPP_CAPTURE=<folder>): it saves every tab, with
// the first play selected, as PNG files and closes the window. It lets the UI be
// checked on a test machine over SSH, where nobody watches the screen.
func (a *app) capture(dir string) {
	shot := func(name string) {
		done := make(chan struct{})
		a.mw.Synchronize(func() {
			defer close(done)
			if img := printWindow(a.mw.Handle()); img != nil {
				if f, err := os.Create(filepath.Join(dir, name+".png")); err == nil {
					_ = png.Encode(f, img)
					f.Close()
				}
			}
		})
		<-done
	}
	time.Sleep(3 * time.Second)
	a.mw.Synchronize(func() {
		if len(a.plays.model.rows) > 0 {
			_ = a.plays.table.SetCurrentIndex(0)
		}
	})
	time.Sleep(time.Second)
	for i := 0; i < a.tabs.Pages().Len(); i++ {
		a.mw.Synchronize(func() { _ = a.tabs.SetCurrentIndex(i) })
		time.Sleep(1500 * time.Millisecond)
		shot(fmt.Sprintf("tab%d", i))
	}
	// The settings tab again, with the hidden advanced group shown (five clicks).
	a.mw.Synchronize(func() {
		_ = a.tabs.SetCurrentIndex(a.tabs.Pages().Len() - 1)
		for range 5 {
			a.settings.versionTap()
		}
	})
	time.Sleep(1500 * time.Millisecond)
	shot("advanced")
	// The install dialog for the first downloaded play; it is closed without
	// starting anything.
	a.mw.Synchronize(func() {
		_ = a.tabs.SetCurrentIndex(0)
		a.plays.onlyReady.SetChecked(true)
		a.plays.apply()
		if pl := a.plays.current(); pl == nil && len(a.plays.model.rows) > 0 {
			_ = a.plays.table.SetCurrentIndex(0)
		}
	})
	time.Sleep(time.Second)
	a.mw.Synchronize(func() { go a.mw.Synchronize(a.plays.flashSelected) })
	time.Sleep(4 * time.Second)
	done := make(chan struct{})
	a.mw.Synchronize(func() {
		defer close(done)
		if d := a.flashDlg; d != nil {
			if img := printWindow(d.dlg.Handle()); img != nil {
				if f, err := os.Create(filepath.Join(dir, "flash.png")); err == nil {
					_ = png.Encode(f, img)
					f.Close()
				}
			}
			d.dlg.Cancel()
		}
	})
	<-done
	time.Sleep(time.Second)
	a.mw.Synchronize(func() { a.mw.Close() })
}

var procPrintWindow = syscall.NewLazyDLL("user32.dll").NewProc("PrintWindow")

// printWindow renders a top-level window with all its child controls
// (PW_RENDERFULLCONTENT) into an image.
func printWindow(hwnd win.HWND) image.Image {
	var r win.RECT
	if !win.GetWindowRect(hwnd, &r) {
		return nil
	}
	w, h := int(r.Right-r.Left), int(r.Bottom-r.Top)
	screen := win.GetDC(0)
	defer win.ReleaseDC(0, screen)
	dc := win.CreateCompatibleDC(screen)
	defer win.DeleteDC(dc)
	bmp := win.CreateCompatibleBitmap(screen, int32(w), int32(h))
	defer win.DeleteObject(win.HGDIOBJ(bmp))
	old := win.SelectObject(dc, win.HGDIOBJ(bmp))
	defer win.SelectObject(dc, old)
	if r, _, _ := procPrintWindow.Call(uintptr(hwnd), uintptr(dc), 2); r == 0 {
		return nil
	}
	var bi win.BITMAPINFO
	bi.BmiHeader.BiSize = uint32(unsafe.Sizeof(bi.BmiHeader))
	bi.BmiHeader.BiWidth = int32(w)
	bi.BmiHeader.BiHeight = -int32(h)
	bi.BmiHeader.BiPlanes = 1
	bi.BmiHeader.BiBitCount = 32
	bi.BmiHeader.BiCompression = win.BI_RGB
	buf := make([]byte, w*h*4)
	if win.GetDIBits(dc, bmp, 0, uint32(h), &buf[0], &bi, win.DIB_RGB_COLORS) == 0 {
		return nil
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(buf); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = buf[i+2], buf[i+1], buf[i], 255
	}
	return img
}
