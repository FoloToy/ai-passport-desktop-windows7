//go:build windows

package ui

import (
	"context"
	"strings"
	"sync/atomic"
	"time"

	"github.com/FoloToy/ai-passport-desktop/pkg/device"
	"github.com/FoloToy/ai-passport-desktop/pkg/flasher"
	"github.com/FoloToy/ai-passport-desktop/pkg/library"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"github.com/FoloToy/ai-passport-desktop-windows7/internal/i18n"
)

// flashDialog installs one play on a device the user picks.
type flashDialog struct {
	a    *app
	play *library.Play
	dlg  *walk.Dialog

	ports    *walk.ComboBox
	portHint *walk.TextLabel
	start    *walk.PushButton
	closeBtn *walk.PushButton
	bar      *walk.ProgressBar
	stage    *walk.Label
	result   *walk.TextLabel
	logView  *walk.TextEdit

	list     []device.Port
	scanning atomic.Bool
	lastScan time.Time
	started  bool
	logs     int
}

func runFlashDialog(a *app, pl *library.Play) {
	d := &flashDialog{a: a, play: pl}
	err := Dialog{
		AssignTo:      &d.dlg,
		Title:         t("flash_title") + " - " + i18n.Text(pl.Title),
		MinSize:       Size{Width: 560, Height: 460},
		Font:          uiFont(9, false),
		Layout:        VBox{},
		DefaultButton: &d.start,
		CancelButton:  &d.closeBtn,
		Children: []Widget{
			Label{Text: plain(i18n.Text(pl.Title)), Font: uiFont(12, true)},
			TextLabel{Text: t("step_connect"), MinSize: Size{Width: 400}},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					Label{Text: t("port_label")},
					ComboBox{AssignTo: &d.ports, StretchFactor: 1},
					PushButton{Text: t("refresh"), OnClicked: func() { d.scan(true) }},
				},
			},
			TextLabel{AssignTo: &d.portHint, TextColor: red, MinSize: Size{Width: 400}},
			TextLabel{Text: sentences(t("flash_hint"), t("flash_warn")), TextColor: grey, MinSize: Size{Width: 400}},
			ProgressBar{AssignTo: &d.bar, MaxValue: 100},
			Label{AssignTo: &d.stage, Font: uiFont(10, true)},
			TextLabel{AssignTo: &d.result, MinSize: Size{Width: 400}},
			Label{Text: t("log"), TextColor: grey},
			TextEdit{AssignTo: &d.logView, ReadOnly: true, VScroll: true, MinSize: Size{Height: 110}},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					HSpacer{},
					PushButton{AssignTo: &d.start, Text: t("start_flash"), OnClicked: d.begin},
					PushButton{AssignTo: &d.closeBtn, Text: t("close"), OnClicked: func() { d.dlg.Cancel() }},
				},
			},
		},
	}.Create(a.mw)
	if err != nil {
		a.showError(err)
		return
	}
	if icon := a.mw.Icon(); icon != nil {
		_ = d.dlg.SetIcon(icon)
	}
	d.dlg.Closing().Attach(func(canceled *bool, _ walk.CloseReason) {
		if a.c.Flash.Active() {
			*canceled = true
			walk.MsgBox(d.dlg, t("device_busy"), t("busy_close"), walk.MsgBoxIconWarning)
		}
	})
	if a.c.Flash.Snapshot().Kind == "flash" {
		a.c.Flash.Reset()
	}
	a.flashDlg = d
	d.scan(true)
	d.refresh()
	d.dlg.Run()
	a.flashDlg = nil
	if st := a.c.Flash.Snapshot(); !st.Active && st.Kind == "flash" {
		a.c.Flash.Reset()
	}
}

// scan lists serial ports in the background; it runs every two seconds while
// the dialog is idle so a device plugged in later shows up by itself.
func (d *flashDialog) scan(force bool) {
	if d.a.c.Flash.Active() || (!force && time.Since(d.lastScan) < 2*time.Second) || !d.scanning.CompareAndSwap(false, true) {
		return
	}
	d.lastScan = time.Now()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		ports, err := d.a.c.Runner.Ports(ctx)
		d.a.mw.Synchronize(func() {
			d.scanning.Store(false)
			if d.dlg == nil || d.a.flashDlg != d {
				return
			}
			if err != nil {
				setText(d.portHint, i18n.ErrText("worker", err.Error()))
				return
			}
			d.setPorts(ports)
		})
	}()
}

func (d *flashDialog) setPorts(ports []device.Port) {
	same := len(ports) == len(d.list)
	for i := range ports {
		if same && ports[i].Path != d.list[i].Path {
			same = false
		}
	}
	recommended := 0
	for _, p := range ports {
		if p.Recommended {
			recommended++
		}
	}
	switch {
	case len(ports) == 0:
		setText(d.portHint, t("no_ports"))
	case recommended == 0:
		setText(d.portHint, t("no_passport"))
	default:
		setText(d.portHint, "")
	}
	if same {
		return
	}
	prev := ""
	if i := d.ports.CurrentIndex(); i >= 0 && i < len(d.list) {
		prev = d.list[i].Path
	}
	d.list = ports
	names := make([]string, len(ports))
	sel := -1
	for i, p := range ports {
		names[i] = p.Path
		if p.Label != "" && p.Label != p.Path {
			names[i] = p.Label + "（" + p.Path + "）"
		}
		if p.Recommended {
			names[i] = "★ " + names[i]
		}
		if p.Path == prev {
			sel = i
		}
	}
	if sel < 0 && recommended == 1 {
		for i, p := range ports {
			if p.Recommended {
				sel = i
			}
		}
	}
	_ = d.ports.SetModel(names)
	_ = d.ports.SetCurrentIndex(sel)
}

func (d *flashDialog) begin() {
	i := d.ports.CurrentIndex()
	if i < 0 || i >= len(d.list) {
		setText(d.portHint, t("choose_port"))
		return
	}
	if err := d.a.c.StartFlash(d.play.Slug, d.list[i].Path); err != nil {
		d.a.showError(err)
		return
	}
	d.started = true
	d.logs = 0
	setText(d.logView, "")
	d.refresh()
}

func (d *flashDialog) refresh() {
	st := d.a.c.Flash.Snapshot()
	active := st.Active
	setEnabled(d.start, !active)
	setEnabled(d.ports, !active)
	setEnabled(d.closeBtn, !active)
	if !active {
		d.scan(false)
	}
	if !d.started || st.Kind != "flash" {
		setText(d.stage, "")
		setText(d.result, "")
		setProgress(d.bar, 0)
		return
	}
	setProgress(d.bar, st.Percent)
	stage, result := flashStageText(st)
	setText(d.stage, stage)
	setText(d.result, result)
	if st.Stage == "failed" {
		d.result.SetTextColor(red)
	} else {
		d.result.SetTextColor(walk.RGB(0, 0, 0))
	}
	if st.Stage == "done" || st.Stage == "failed" {
		setText(d.start, t("retry"))
	}
	if len(st.Logs) != d.logs {
		d.logs = len(st.Logs)
		d.logView.SetText(crlf(strings.Join(st.Logs, "\n")))
		d.logView.SetTextSelection(len(d.logView.Text()), len(d.logView.Text()))
	}
}

// flashStageText is the headline and the detail of an install.
func flashStageText(st flasher.State) (string, string) {
	switch st.Stage {
	case "done":
		sub := t("flash_done_sub")
		if !st.Restarted {
			sub = t("flash_done_replug")
		}
		if st.CardIDOverwritten {
			sub += "\r\n" + t("cardid_overwritten")
		}
		return t("flash_done"), sub
	case "failed":
		return t("flash_failed"), i18n.ErrText(st.ErrorCode, st.Error)
	case "", "idle":
		return "", ""
	}
	return stageName(st.Stage), ""
}

// stageName translates a device job stage.
func stageName(stage string) string {
	if i18n.Has("stage_" + stage) {
		return t("stage_" + stage)
	}
	return stage
}
