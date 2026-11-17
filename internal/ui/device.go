//go:build windows

package ui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

// knownDevice is a device this computer installed, backed up or restored, kept
// in devices.json in the private settings folder (the desktop app keeps the same
// list in the browser storage of its window).
type knownDevice struct {
	MAC   string        `json:"mac"`
	Flash string        `json:"flash,omitempty"`
	Last  *library.Text `json:"last,omitempty"`
	At    time.Time     `json:"at"`
}

func (a *app) devicesFile() string { return filepath.Join(a.c.StateDir, "devices.json") }

func (a *app) knownDevices() []knownDevice {
	var list []knownDevice
	if b, err := os.ReadFile(a.devicesFile()); err == nil {
		_ = json.Unmarshal(b, &list)
	}
	return list
}

func (a *app) saveDevices(list []knownDevice) {
	if len(list) > 20 {
		list = list[:20]
	}
	if b, err := json.MarshalIndent(list, "", " "); err == nil {
		_ = os.WriteFile(a.devicesFile(), b, 0o600)
	}
	a.device.showDevices()
}

// rememberDevice records the device of a finished job.
func (a *app) rememberDevice(st flasher.State) {
	if st.MAC == "" {
		return
	}
	old := a.knownDevices()
	prev := knownDevice{}
	list := []knownDevice{}
	for _, d := range old {
		if d.MAC == st.MAC {
			prev = d
		} else {
			list = append(list, d)
		}
	}
	d := knownDevice{MAC: st.MAC, Flash: st.Flash, Last: prev.Last, At: time.Now()}
	if d.Flash == "" {
		d.Flash = prev.Flash
	}
	if st.Kind == "flash" && st.Slug != "" {
		if pl, err := a.c.Play(st.Slug); err == nil && pl != nil {
			title := pl.Title
			d.Last = &title
		}
	}
	a.saveDevices(append([]knownDevice{d}, list...))
}

type devicePage struct {
	a   *app
	tab int

	usbState *walk.Label
	usbSub   *walk.TextLabel
	devices  *walk.ListBox
	forget   *walk.PushButton
	empty    *walk.TextLabel
	list     []knownDevice

	scanning atomic.Bool
	lastScan time.Time
}

func (p *devicePage) page() TabPage {
	wrap := Size{Width: 360}
	help := []string{}
	for i := 1; i <= 5; i++ {
		help = append(help, "•  "+t("help_item"+string(rune('0'+i))))
	}
	return TabPage{
		Title:  t("nav_device"),
		Layout: HBox{Alignment: AlignHNearVNear},
		Children: []Widget{
			Composite{
				StretchFactor: 1,
				Layout:        VBox{MarginsZero: true},
				Children: []Widget{
					Label{Text: t("dev_title"), Font: uiFont(13, true)},
					TextLabel{Text: t("dev_lead"), MinSize: wrap},
					GroupBox{
						Title:  t("usb_title"),
						Layout: VBox{},
						Children: []Widget{
							Label{AssignTo: &p.usbState, Font: uiFont(11, true), Text: t("usb_checking")},
							TextLabel{AssignTo: &p.usbSub, TextColor: grey, MinSize: wrap},
							Composite{Layout: Grid{Columns: 2, MarginsZero: true}, Children: []Widget{
								PushButton{Text: t("dev_go_plays"), OnClicked: func() { p.a.goTab(p.a.plays.tab) }},
								PushButton{Text: t("dev_local"), OnClicked: p.a.tools.flashLocal},
								PushButton{Text: t("dev_backup"), OnClicked: func() { p.a.goTab(p.a.tools.tab); p.a.tools.backup() }},
								PushButton{Text: t("dev_restore"), OnClicked: func() { p.a.goTab(p.a.tools.tab); p.a.tools.restore() }},
							}},
						},
					},
					GroupBox{
						Title:  t("my_devices"),
						Layout: VBox{},
						Children: []Widget{
							TextLabel{AssignTo: &p.empty, Text: t("my_devices_empty"), TextColor: grey, MinSize: wrap},
							ListBox{AssignTo: &p.devices, MinSize: Size{Height: 110}, OnCurrentIndexChanged: func() { setEnabled(p.forget, p.devices.CurrentIndex() >= 0) }},
							Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
								HSpacer{},
								PushButton{AssignTo: &p.forget, Text: t("dev_remove"), Enabled: false, OnClicked: p.forgetSelected},
							}},
						},
					},
					VSpacer{},
				},
			},
			Composite{
				StretchFactor: 1,
				Layout:        VBox{MarginsZero: true},
				Children: []Widget{
					GroupBox{
						Title:  t("ble_title") + "（" + t("needs_ble") + "）",
						Layout: VBox{},
						Children: []Widget{
							TextLabel{Text: t("ble_lead"), MinSize: wrap},
						},
					},
					GroupBox{
						Title:  t("help_title"),
						Layout: VBox{},
						Children: []Widget{
							TextLabel{Text: strings.Join(help, "\r\n"), MinSize: wrap},
						},
					},
					VSpacer{},
				},
			},
		},
	}
}

func (p *devicePage) init() { p.showDevices() }

func (p *devicePage) showDevices() {
	if p.devices == nil {
		return
	}
	p.list = p.a.knownDevices()
	names := make([]string, len(p.list))
	for i, d := range p.list {
		parts := []string{"AI Passport · " + d.MAC}
		if d.Flash != "" {
			parts = append(parts, d.Flash)
		}
		if d.Last != nil {
			parts = append(parts, i18n.Tf("dev_last_play_n", i18n.Text(*d.Last)))
		}
		parts = append(parts, i18n.Tf("dev_seen_n", d.At.Local().Format("2006-01-02")))
		names[i] = strings.Join(parts, "  ·  ")
	}
	_ = p.devices.SetModel(names)
	setVisible(p.empty, len(names) == 0)
	setVisible(p.devices, len(names) > 0)
	setEnabled(p.forget, false)
}

func (p *devicePage) forgetSelected() {
	i := p.devices.CurrentIndex()
	if i < 0 || i >= len(p.list) {
		return
	}
	list := append(append([]knownDevice{}, p.list[:i]...), p.list[i+1:]...)
	p.a.saveDevices(list)
}

// refresh checks the USB ports every three seconds while the tab is shown.
func (p *devicePage) refresh() {
	if p.a.tabs.CurrentIndex() != p.a.device.tab {
		return
	}
	if st := p.a.c.Flash.Snapshot(); st.Active {
		setText(p.usbState, t("usb_connected"))
		setText(p.usbSub, st.Port)
		return
	}
	if time.Since(p.lastScan) < 3*time.Second || !p.scanning.CompareAndSwap(false, true) {
		return
	}
	p.lastScan = time.Now()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		ports, err := p.a.c.Runner.Ports(ctx)
		p.a.mw.Synchronize(func() {
			p.scanning.Store(false)
			if err != nil {
				return
			}
			p.showPorts(ports)
		})
	}()
}

func (p *devicePage) showPorts(ports []device.Port) {
	var rec []device.Port
	for _, x := range ports {
		if x.Recommended {
			rec = append(rec, x)
		}
	}
	switch {
	case len(rec) == 1:
		setText(p.usbState, t("usb_connected"))
		setText(p.usbSub, rec[0].Path)
		p.usbState.SetTextColor(green)
	case len(rec) > 1:
		setText(p.usbState, t("usb_connected"))
		setText(p.usbSub, i18n.Tf("usb_many_n", len(rec)))
		p.usbState.SetTextColor(red)
	case p.a.driverMissing:
		setText(p.usbState, t("usb_unknown"))
		setText(p.usbSub, t("driver_missing_short"))
		p.usbState.SetTextColor(red)
	case len(ports) > 0:
		setText(p.usbState, t("usb_unknown"))
		setText(p.usbSub, t("usb_unknown_sub"))
		p.usbState.SetTextColor(grey)
	default:
		setText(p.usbState, t("usb_none"))
		setText(p.usbSub, t("usb_none_sub"))
		p.usbState.SetTextColor(grey)
	}
}
