//go:build windows

package ui

import (
	"sort"
	"strconv"
	"time"

	"github.com/FoloToy/ai-passport-desktop/pkg/factory"
	"github.com/FoloToy/ai-passport-desktop/pkg/library"
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

	// Hidden batch download for show booths, shown after five clicks on the
	// version line in About.
	advanced  *walk.GroupBox
	batchText *walk.Label
	batchBtn  *walk.PushButton
	taps      []time.Time

	usageFor  string
	usageNext time.Time
	usageDone chan string
}

// widgets are the settings groups; they sit on the right half of 我的, as the
// settings are part of the Me tab in the Android app.
func (p *settingsPage) widgets() []Widget {
	origin := p.a.c.Sync.Client.Origin
	return []Widget{
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
		GroupBox{
			AssignTo:  &p.advanced,
			Title:     t("batch_title"),
			Layout:    VBox{},
			Alignment: AlignHNearVNear,
			Visible:   false,
			Children: []Widget{
				TextLabel{Text: i18n.Tf("batch_lead_n", batchSize), MinSize: Size{Width: 300}},
				Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
					PushButton{AssignTo: &p.batchBtn, Text: t("batch_start"), OnClicked: p.toggleBatch},
					Label{AssignTo: &p.batchText, TextColor: grey},
					HSpacer{},
				}},
			},
		},
		VSpacer{},
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
}

// versionTap shows or hides the batch download after five clicks within three seconds.
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
	on := !p.advanced.Visible() || p.a.c.Sync.Batch().Running
	setVisible(p.advanced, on)
	if on {
		setText(p.a.status, t("hidden_on"))
	} else {
		setText(p.a.status, t("hidden_off"))
	}
}

// batchSize is how many plays the booth download takes: the first screen of the
// library, as in the desktop app.
const batchSize = 16

// firstScreen is the first screen of the library: the recommended plays (filled
// up from the must-play topic, at most 12), then the newest plays.
func firstScreen(idx *library.Index) []string {
	if idx == nil {
		return nil
	}
	byID := map[int]*library.Play{}
	for i := range idx.Plays {
		byID[idx.Plays[i].ID] = &idx.Plays[i]
	}
	var featured []*library.Play
	seen := map[*library.Play]bool{}
	add := func(pl *library.Play) {
		if pl != nil && !pl.Removed && !seen[pl] {
			seen[pl] = true
			featured = append(featured, pl)
		}
	}
	for _, id := range idx.Recommendations {
		add(byID[id])
	}
	if len(featured) < 4 {
		for _, tp := range idx.Topics {
			if tp.Key == "must-play" {
				for _, id := range tp.IDs {
					add(byID[id])
				}
			}
		}
	}
	if len(featured) > 12 {
		featured = featured[:12]
	}
	newest := make([]*library.Play, 0, len(idx.Plays))
	for i := range idx.Plays {
		if !idx.Plays[i].Removed {
			newest = append(newest, &idx.Plays[i])
		}
	}
	sort.SliceStable(newest, func(i, j int) bool { return newest[i].PublishedAt > newest[j].PublishedAt })
	var out []string
	taken := map[string]bool{}
	for _, pl := range append(featured, newest...) {
		if len(out) >= batchSize {
			break
		}
		if !pl.Removed && (pl.Ready || pl.Downloadable) && !taken[pl.Slug] {
			taken[pl.Slug] = true
			out = append(out, pl.Slug)
		}
	}
	return out
}

func (p *settingsPage) toggleBatch() {
	c := p.a.c
	if c.Sync.Batch().Running {
		c.Sync.CancelBatch()
		return
	}
	idx, _, _ := c.Index()
	slugs := firstScreen(idx)
	if len(slugs) == 0 {
		walk.MsgBox(p.a.mw, t("batch_title"), t("batch_none"), walk.MsgBoxIconWarning)
		return
	}
	p.a.showError(c.StartBatch(slugs))
}

func (p *settingsPage) refreshBatch() {
	b := p.a.c.Sync.Batch()
	if b.Running {
		setVisible(p.advanced, true)
		setText(p.batchBtn, t("batch_stop"))
	} else {
		setText(p.batchBtn, t("batch_start"))
	}
	text := ""
	switch {
	case b.Running && !b.ThrottledUntil.IsZero() && time.Until(b.ThrottledUntil) > 0:
		text = i18n.Tf("batch_running_n", b.Done, b.Total) + "    " + i18n.Tf("batch_wait_n", wait(b.ThrottledUntil))
	case b.Running:
		text = i18n.Tf("batch_running_n", b.Done, b.Total)
	case b.Total > 0:
		text = i18n.Tf("batch_done_n", b.Done, b.Total)
		if b.Failed > 0 {
			text += i18n.Tf("batch_failed_n", b.Failed)
		}
	}
	setText(p.batchText, text)
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
	p.refreshBatch()
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
