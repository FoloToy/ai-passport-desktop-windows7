//go:build windows

package ui

import (
	"strconv"
	"time"

	"github.com/FoloToy/ai-passport-desktop/pkg/factory"
	"github.com/FoloToy/ai-passport-desktop/pkg/syncer"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"github.com/FoloToy/ai-passport-desktop-windows7/internal/i18n"
)

type settingsPage struct {
	a *app

	dir       *walk.LineEdit
	usage     *walk.Label
	data      *walk.Label
	update    *walk.Label
	checkBtn  *walk.PushButton
	lang      *walk.ComboBox
	langNote  *walk.Label
	langReady bool

	// Hidden "advanced" group, shown after five clicks on the version line.
	advanced  *walk.GroupBox
	prefetch  *walk.CheckBox
	taps      []time.Time
	prefReady bool

	usageFor  string
	usageNext time.Time
	usageDone chan string
}

func (p *settingsPage) page() TabPage {
	origin := p.a.c.Sync.Client.Origin
	return TabPage{
		Title:  t("nav_settings"),
		Layout: VBox{},
		Children: []Widget{
			GroupBox{
				Title:  t("s_library"),
				Layout: VBox{},
				Children: []Widget{
					Label{Text: t("s_library_sub"), TextColor: grey},
					Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
						LineEdit{AssignTo: &p.dir, ReadOnly: true},
						PushButton{Text: t("change_dir"), OnClicked: p.changeDir},
						PushButton{Text: t("reveal_win"), OnClicked: func() {
							if dir, err := p.a.c.RevealPath("library", ""); err == nil {
								reveal(dir)
							}
						}},
					}},
					Label{AssignTo: &p.usage, TextColor: grey},
				},
			},
			GroupBox{
				Title:  t("s_data"),
				Layout: VBox{},
				Children: []Widget{
					Label{AssignTo: &p.data},
					Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
						PushButton{AssignTo: &p.checkBtn, Text: t("check_update"), OnClicked: p.a.c.CheckUpdatesInBackground},
						Label{AssignTo: &p.update},
						HSpacer{},
					}},
				},
			},
			GroupBox{
				Title:  t("s_language"),
				Layout: HBox{},
				Children: []Widget{
					ComboBox{AssignTo: &p.lang, Model: []string{t("lang_auto"), t("lang_zh"), t("lang_en")}, OnCurrentIndexChanged: p.changeLang},
					Label{AssignTo: &p.langNote, Text: t("lang_restart"), TextColor: grey, Visible: false},
					HSpacer{},
				},
			},
			GroupBox{
				AssignTo:  &p.advanced,
				Title:     t("s_advanced"),
				Layout:    VBox{},
				Alignment: AlignHNearVNear,
				Visible:   false,
				Children: []Widget{
					Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
						CheckBox{AssignTo: &p.prefetch, Text: t("s_prefetch"), OnCheckedChanged: p.changePrefetch},
						HSpacer{},
					}},
					TextLabel{Text: t("s_prefetch_sub"), TextColor: grey, MinSize: Size{Width: 400}},
				},
			},
			GroupBox{
				Title:     t("s_about"),
				Layout:    VBox{},
				Alignment: AlignHNearVNear,
				Children: []Widget{
					Label{
						Text:        t("app_title") + "（" + t("edition") + "）  " + t("version") + " " + p.a.c.Version,
						OnMouseDown: func(x, y int, button walk.MouseButton) { p.versionTap() },
					},
					Label{Text: t("factory_fw") + "：" + factory.Version, TextColor: grey},
					LinkLabel{
						Text:            `<a href="` + origin + `/plays/">` + t("website") + `</a>    <a href="https://github.com/FoloToy/ai-passport-desktop-windows7">GitHub</a>`,
						OnLinkActivated: func(link *walk.LinkLabelLink) { openURL(link.URL()) },
					},
				},
			},
			VSpacer{},
		},
	}
}

func (p *settingsPage) init() {
	p.usageDone = make(chan string, 1)
	switch p.a.c.Settings.Get().Language {
	case "zh":
		_ = p.lang.SetCurrentIndex(1)
	case "en":
		_ = p.lang.SetCurrentIndex(2)
	default:
		_ = p.lang.SetCurrentIndex(0)
	}
	setVisible(p.langNote, false)
	p.langReady = true
	on := p.a.c.Settings.Get().PrefetchFirmware
	p.prefetch.SetChecked(on)
	setVisible(p.advanced, on) // keep it visible while the setting is on
	p.prefReady = true
}

// versionTap shows or hides the advanced group after five clicks within three seconds.
func (p *settingsPage) versionTap() {
	now := time.Now()
	taps := p.taps[:0]
	for _, at := range p.taps {
		if now.Sub(at) < 3*time.Second {
			taps = append(taps, at)
		}
	}
	p.taps = append(taps, now)
	if len(p.taps) < 5 {
		return
	}
	p.taps = nil
	setVisible(p.advanced, !p.advanced.Visible())
}

func (p *settingsPage) changePrefetch() {
	if !p.prefReady {
		return
	}
	p.a.showError(p.a.c.SetPrefetchFirmware(p.prefetch.Checked()))
}

func (p *settingsPage) changeLang() {
	if !p.langReady {
		return
	}
	lang := []string{"", "zh", "en"}[max(0, p.lang.CurrentIndex())]
	if lang == p.a.c.Settings.Get().Language {
		return
	}
	p.a.showError(p.a.c.SetLanguage(lang))
	setVisible(p.langNote, true)
}

// changeDir picks a new library folder and asks whether to move the plays there.
func (p *settingsPage) changeDir() {
	c := p.a.c
	cur := c.Settings.Get().LibraryDir
	dir, ok := chooseFolder(p.a.mw, t("choose_library"), cur)
	if !ok || dir == cur {
		return
	}
	mode := "switch"
	if c.HasLibraryData() {
		msg := t("move_q") + "\r\n\r\n" + "“" + t("move_yes") + "”→ " + yesNo(true) + "\r\n“" + t("move_no") + "”→ " + yesNo(false) + "（" + t("move_no_sub") + "）"
		switch walk.MsgBox(p.a.mw, t("change_dir"), msg, walk.MsgBoxYesNoCancel|walk.MsgBoxIconQuestion) {
		case walk.DlgCmdYes:
			mode = "move"
		case walk.DlgCmdNo:
		default:
			return
		}
	}
	if err := c.SetLibrary(dir, mode); err != nil {
		p.a.showError(err)
		return
	}
	p.usageNext = time.Time{}
	p.a.plays.reload()
}

func yesNo(yes bool) string {
	if i18n.Current() == "en" {
		if yes {
			return "Yes"
		}
		return "No"
	}
	if yes {
		return "是"
	}
	return "否"
}

func (p *settingsPage) refresh() {
	c := p.a.c
	info := c.LibraryInfo()
	setText(p.dir, info.Dir)

	// The folder size is counted in the background; it can take a moment.
	select {
	case s := <-p.usageDone:
		setText(p.usage, s)
	default:
	}
	if info.Dir != "" && (info.Dir != p.usageFor || time.Now().After(p.usageNext)) && !c.Sync.Running() {
		p.usageFor, p.usageNext = info.Dir, time.Now().Add(30*time.Second)
		lib := c.Library()
		go func() {
			size, files := lib.Usage()
			s := t("usage") + " " + i18n.Bytes(size) + "（" + strconv.Itoa(files) + " " + t("files") + "）"
			if free, ok := syncer.FreeSpace(lib.Root); ok {
				s += "    " + t("free") + " " + i18n.Bytes(int64(free))
			}
			select {
			case p.usageDone <- s:
			default:
			}
		}()
	}

	data := t("never")
	if info.HasIndex {
		data = i18n.Tf("plays_summary", info.Plays, info.Ready)
		if !info.SyncedAt.IsZero() {
			data += "    " + i18n.Tf("synced_at", info.SyncedAt.Local().Format("2006-01-02 15:04"))
		}
		if info.Complete {
			data += "    " + t("status") + "：" + t("complete")
		} else {
			data += "    " + t("status") + "：" + t("incomplete")
		}
	}
	setText(p.data, data)

	up := c.Sync.Update()
	setEnabled(p.checkBtn, info.HasIndex && !up.Checking)
	switch {
	case up.Checking:
		setText(p.update, t("checking"))
	case up.Error != "":
		setText(p.update, i18n.ErrText("network", up.Error))
	case up.Available:
		setText(p.update, i18n.BannerUpdate(up.Changes))
	case !up.CheckedAt.IsZero():
		setText(p.update, t("up_to_date"))
	default:
		setText(p.update, "")
	}
}
