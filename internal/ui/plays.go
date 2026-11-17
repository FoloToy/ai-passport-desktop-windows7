//go:build windows

package ui

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/FoloToy/ai-passport-desktop/pkg/library"
	"github.com/FoloToy/ai-passport-desktop/pkg/settings"
	"github.com/FoloToy/ai-passport-desktop/pkg/syncer"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"github.com/FoloToy/ai-passport-desktop-windows7/internal/i18n"
)

// playModel is the table of plays after search, filter and sort.
type playModel struct {
	walk.TableModelBase
	rows  []*library.Play
	ready map[string]bool
	cats  map[string]string
}

func (m *playModel) RowCount() int { return len(m.rows) }

func (m *playModel) Value(row, col int) interface{} {
	p := m.rows[row]
	switch col {
	case 0:
		return i18n.Text(p.Title)
	case 1:
		return m.cats[p.Category]
	case 2:
		return p.Author.Name
	case 3:
		return p.Downloads
	case 4:
		return day(p.PublishedAt)
	case 5:
		return firmwareStatus(p, m.ready[p.Slug])
	}
	return ""
}

func firmwareStatus(p *library.Play, ready bool) string {
	switch {
	case p.Ready || ready:
		return t("downloaded")
	case !p.Downloadable:
		return "—"
	}
	return t("not_ready")
}

func day(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// filter is one entry of the category list: a category key or a topic.
type filter struct {
	name     string
	category string
	topic    map[int]bool
}

type playsPage struct {
	a   *app
	tab int

	// Toolbar and sync status.
	search    *walk.LineEdit
	category  *walk.ComboBox
	sortBy    *walk.ComboBox
	onlyReady *walk.CheckBox
	syncBtn   *walk.PushButton
	syncBox   *walk.Composite
	syncText  *walk.Label
	syncBar   *walk.ProgressBar
	syncInfo  *walk.Label
	banner    *walk.Label

	// Welcome screen, shown until the first download finishes.
	welcome   *walk.Composite
	folder    *walk.LineEdit
	browse    *walk.Splitter
	table     *walk.TableView
	model     *playModel
	filters   []filter
	index     *library.Index
	readyVer  int
	lastSync  string
	selected  string
	lastCover *walk.Bitmap

	// Detail panel.
	detail      *walk.Composite
	empty       *walk.Label
	cover       *walk.ImageView
	title       *walk.Label
	meta        *walk.TextLabel
	fwInfo      *walk.Label
	flashBtn    *walk.PushButton
	fetchBtn    *walk.PushButton
	intro       *walk.TextEdit
	usage       *walk.TextEdit
	changelog   *walk.TextEdit
	fetchStatus *walk.Label

	// Likes, favorites and comments, live from the community site.
	heartBtn   *walk.PushButton
	favBtn     *walk.PushButton
	commentBtn *walk.PushButton
	viewTimer  *time.Timer
}

func (p *playsPage) page() TabPage {
	p.model = &playModel{ready: map[string]bool{}, cats: map[string]string{}}
	bold := uiFont(12, true)
	return TabPage{
		Title:  t("nav_plays"),
		Layout: VBox{},
		Children: []Widget{
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					LineEdit{AssignTo: &p.search, CueBanner: t("search_ph"), MinSize: Size{Width: 240}, MaxSize: Size{Width: 380}, OnTextChanged: p.apply},
					ComboBox{AssignTo: &p.category, MinSize: Size{Width: 170}, OnCurrentIndexChanged: p.apply},
					ComboBox{AssignTo: &p.sortBy, Model: []string{t("sort_newest"), t("sort_downloads"), t("sort_hearts")}, CurrentIndex: 0, OnCurrentIndexChanged: p.apply},
					CheckBox{AssignTo: &p.onlyReady, Text: t("only_ready"), OnCheckedChanged: p.apply},
					HSpacer{},
					Label{AssignTo: &p.banner, TextColor: green, Visible: false},
					PushButton{AssignTo: &p.syncBtn, Text: t("sync_title"), OnClicked: p.toggleSync},
				},
			},
			Composite{
				AssignTo: &p.syncBox,
				Visible:  false,
				Layout:   HBox{MarginsZero: true},
				Children: []Widget{
					Label{AssignTo: &p.syncText},
					ProgressBar{AssignTo: &p.syncBar, MaxValue: 100, MinSize: Size{Width: 240}},
					Label{AssignTo: &p.syncInfo, TextColor: grey},
					HSpacer{},
				},
			},
			Composite{
				AssignTo: &p.welcome,
				Visible:  false,
				Layout:   VBox{Margins: Margins{Left: 40, Top: 30, Right: 40, Bottom: 30}},
				Children: []Widget{
					Label{Text: t("welcome_title"), Font: uiFont(14, true)},
					TextLabel{Text: t("welcome_sub"), MinSize: Size{Width: 400}},
					VSpacer{Size: 10},
					Label{Text: t("folder_label")},
					Composite{
						Layout: HBox{MarginsZero: true},
						Children: []Widget{
							LineEdit{AssignTo: &p.folder, ReadOnly: true},
							PushButton{Text: t("change"), OnClicked: p.chooseFirstFolder},
						},
					},
					Composite{
						Layout: HBox{MarginsZero: true},
						Children: []Widget{
							PushButton{Text: t("start_update"), OnClicked: p.firstSync, MinSize: Size{Width: 160}},
							HSpacer{},
						},
					},
					VSpacer{},
				},
			},
			HSplitter{
				AssignTo:      &p.browse,
				StretchFactor: 1,
				Children: []Widget{
					TableView{
						AssignTo:            &p.table,
						StretchFactor:       3,
						AlternatingRowBG:    true,
						LastColumnStretched: true,
						Columns: []TableViewColumn{
							{Title: t("col_name"), Width: 230},
							{Title: t("col_category"), Width: 110},
							{Title: t("col_author"), Width: 110},
							{Title: t("col_downloads"), Width: 70, Alignment: AlignFar},
							{Title: t("col_updated"), Width: 90},
							{Title: t("col_status"), Width: 70},
						},
						Model:                 p.model,
						OnCurrentIndexChanged: p.selectRow,
						OnItemActivated:       p.flashSelected,
					},
					Composite{
						StretchFactor: 2,
						MinSize:       Size{Width: 400},
						Layout:        VBox{MarginsZero: true},
						Children: []Widget{
							Label{AssignTo: &p.empty, Text: t("select_play"), TextColor: grey},
							Composite{
								AssignTo: &p.detail,
								Visible:  false,
								Layout:   VBox{MarginsZero: true},
								Children: []Widget{
									ImageView{AssignTo: &p.cover, Mode: ImageViewModeZoom, MinSize: Size{Width: 320, Height: 180}, MaxSize: Size{Height: 180}},
									Label{AssignTo: &p.title, Font: bold},
									TextLabel{AssignTo: &p.meta, TextColor: grey, MinSize: Size{Width: 200}},
									Composite{
										Layout: HBox{MarginsZero: true},
										Children: []Widget{
											PushButton{AssignTo: &p.heartBtn, OnClicked: func() {
												if pl := p.current(); pl != nil {
													p.a.toggleHeart(pl)
												}
											}},
											PushButton{AssignTo: &p.favBtn, OnClicked: func() {
												if pl := p.current(); pl != nil {
													p.a.toggleFavorite(pl)
												}
											}},
											PushButton{AssignTo: &p.commentBtn, OnClicked: func() {
												if pl := p.current(); pl != nil {
													runComments(p.a, pl)
												}
											}},
											HSpacer{},
										},
									},
									Label{AssignTo: &p.fwInfo},
									Composite{
										Layout: HBox{MarginsZero: true},
										Children: []Widget{
											PushButton{AssignTo: &p.flashBtn, Text: t("flash_device"), OnClicked: p.flashSelected},
											PushButton{AssignTo: &p.fetchBtn, Text: t("fetch_btn"), OnClicked: p.fetchSelected},
											PushButton{Text: t("open_folder"), OnClicked: func() {
												if dir, err := p.a.c.RevealPath("play", p.selected); err == nil {
													reveal(dir)
												}
											}},
											PushButton{Text: t("open_web"), OnClicked: func() {
												if url, err := p.a.c.PlayURL(p.selected); err == nil {
													openURL(url)
												}
											}},
											HSpacer{},
										},
									},
									Label{AssignTo: &p.fetchStatus, TextColor: grey},
									TabWidget{
										Pages: []TabPage{
											{Title: t("tab_intro"), Layout: VBox{MarginsZero: true}, Children: []Widget{TextEdit{AssignTo: &p.intro, ReadOnly: true, VScroll: true}}},
											{Title: t("tab_usage"), Layout: VBox{MarginsZero: true}, Children: []Widget{TextEdit{AssignTo: &p.usage, ReadOnly: true, VScroll: true}}},
											{Title: t("tab_changelog"), Layout: VBox{MarginsZero: true}, Children: []Widget{TextEdit{AssignTo: &p.changelog, ReadOnly: true, VScroll: true}}},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (p *playsPage) init() {
	dir := p.a.c.Settings.Get().LibraryDir
	if dir == "" {
		dir = settings.DefaultLibraryDir()
	}
	setText(p.folder, dir)
	p.reload()
}

// reload reads the library index again (after a sync or a folder change).
func (p *playsPage) reload() {
	idx, _, err := p.a.c.Index()
	if err != nil {
		idx = nil
	}
	p.index = idx
	setVisible(p.welcome, idx == nil)
	setVisible(p.browse, idx != nil)
	if idx == nil {
		return
	}
	p.model.cats = map[string]string{}
	p.filters = []filter{{name: t("all_categories")}}
	for _, c := range idx.Categories {
		name := i18n.Text(c.Name)
		p.model.cats[c.Key] = name
		p.filters = append(p.filters, filter{name: name + "（" + strconv.Itoa(c.Count) + "）", category: c.Key})
	}
	for _, tp := range idx.Topics {
		ids := map[int]bool{}
		for _, id := range tp.IDs {
			ids[id] = true
		}
		p.filters = append(p.filters, filter{name: t("topics") + "：" + i18n.Text(tp.Name), topic: ids})
	}
	names := make([]string, len(p.filters))
	for i, f := range p.filters {
		names[i] = f.name
	}
	cur := p.category.CurrentIndex()
	_ = p.category.SetModel(names)
	if cur < 0 || cur >= len(names) {
		cur = 0
	}
	_ = p.category.SetCurrentIndex(cur)
	p.apply()
}

// apply filters and sorts the plays into the table, keeping the selection.
func (p *playsPage) apply() {
	if p.index == nil || p.model == nil || p.table == nil {
		return
	}
	q := strings.ToLower(strings.TrimSpace(p.search.Text()))
	f := filter{}
	if i := p.category.CurrentIndex(); i >= 0 && i < len(p.filters) {
		f = p.filters[i]
	}
	only := p.onlyReady.Checked()
	rows := p.model.rows[:0:0]
	for i := range p.index.Plays {
		pl := &p.index.Plays[i]
		if pl.Removed && !pl.Ready {
			continue
		}
		if f.category != "" && pl.Category != f.category {
			continue
		}
		if f.topic != nil && !f.topic[pl.ID] {
			continue
		}
		if only && !(pl.Ready || p.model.ready[pl.Slug]) {
			continue
		}
		if q != "" && !matches(pl, q) {
			continue
		}
		rows = append(rows, pl)
	}
	switch p.sortBy.CurrentIndex() {
	case 1:
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Downloads > rows[j].Downloads })
	case 2:
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Hearts > rows[j].Hearts })
	default:
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].PublishedAt > rows[j].PublishedAt })
	}
	p.model.rows = rows
	p.model.PublishRowsReset()
	sel := -1
	for i, r := range rows {
		if r.Slug == p.selected {
			sel = i
			break
		}
	}
	_ = p.table.SetCurrentIndex(sel)
	if sel < 0 {
		p.show(nil)
	}
}

func matches(p *library.Play, q string) bool {
	for _, s := range []string{p.Title.Zh, p.Title.En, p.Description.Zh, p.Description.En, p.Author.Name, p.Slug} {
		if strings.Contains(strings.ToLower(s), q) {
			return true
		}
	}
	return false
}

func (p *playsPage) current() *library.Play {
	i := p.table.CurrentIndex()
	if i < 0 || i >= len(p.model.rows) {
		return nil
	}
	return p.model.rows[i]
}

func (p *playsPage) selectRow() { p.show(p.current()) }

// show fills the detail panel.
func (p *playsPage) show(pl *library.Play) {
	setVisible(p.detail, pl != nil)
	setVisible(p.empty, pl == nil)
	if pl == nil {
		p.selected = ""
		return
	}
	changed := pl.Slug != p.selected
	p.selected = pl.Slug
	if changed {
		p.viewed(pl)
		var bmp *walk.Bitmap
		lib := p.a.c.Library()
		for _, rel := range []string{pl.Cover, pl.Thumb} {
			if path, ok := lib.Resolve(rel); ok && rel != "" {
				if bmp = loadImage(path, p.a.mw.DPI()); bmp != nil {
					break
				}
			}
		}
		_ = p.cover.SetImage(bmp)
		if p.lastCover != nil {
			p.lastCover.Dispose()
		}
		p.lastCover = bmp
		setVisible(p.cover, bmp != nil)
		setText(p.intro, crlf(i18n.Text(pl.Description)))
		setText(p.usage, crlf(i18n.Text(pl.Instructions)))
		setText(p.changelog, crlf(i18n.Text(pl.Changelog)))
	}
	title := i18n.Text(pl.Title)
	if pl.Removed {
		title += "（" + t("removed") + "）"
	}
	setText(p.title, plain(title))
	meta := []string{}
	if pl.Author.Name != "" {
		author := pl.Author.Name
		if pl.Official || pl.Author.Official {
			author += "（" + t("official") + "）"
		}
		meta = append(meta, author)
	}
	if c := p.model.cats[pl.Category]; c != "" {
		meta = append(meta, c)
	}
	if d := day(pl.UpdatedAt); d != "" {
		meta = append(meta, t("updated_at")+" "+d)
	} else if d := day(pl.PublishedAt); d != "" {
		meta = append(meta, t("published_at")+" "+d)
	}
	meta = append(meta, t("downloads")+" "+strconv.Itoa(pl.Downloads))
	setText(p.meta, plain(strings.Join(meta, "  ·  ")))
	p.refreshEngagement()
	p.refreshFirmware(pl)
}

func (p *playsPage) refreshFirmware(pl *library.Play) {
	ready := pl.Ready || p.model.ready[pl.Slug] || p.a.c.FirmwareReady(pl.Slug)
	fw := t("firmware") + "："
	switch {
	case ready:
		fw += t("downloaded")
		if pl.Firmware.Size > 0 {
			fw += "，" + t("size") + " " + i18n.Bytes(pl.Firmware.Size)
		}
	case !pl.Downloadable:
		fw += "—"
	default:
		fw += t("not_ready")
	}
	setText(p.fwInfo, fw)
	busy := p.a.c.Flash.Active()
	setEnabled(p.flashBtn, ready && !busy)
	fetch := p.a.c.Sync.Fetch()
	fetching := fetch.Running && fetch.Slug == pl.Slug
	setVisible(p.fetchBtn, !ready && pl.Downloadable)
	setEnabled(p.fetchBtn, !fetching)
	status := ""
	if fetch.Slug == pl.Slug {
		switch {
		case fetching && !fetch.ThrottledUntil.IsZero() && time.Until(fetch.ThrottledUntil) > 0:
			status = i18n.FetchWait(wait(fetch.ThrottledUntil))
		case fetching:
			status = t("fetching")
		case fetch.ErrorCode != "" || fetch.Error != "":
			status = i18n.ErrText(fetch.ErrorCode, fetch.Error)
		case ready:
			status = t("fetch_done")
		}
	}
	setText(p.fetchStatus, status)
}

func wait(until time.Time) string {
	d := time.Until(until).Round(time.Second)
	if d < 0 {
		d = 0
	}
	if d >= time.Minute {
		return strconv.Itoa(int(d.Minutes())) + " min"
	}
	return strconv.Itoa(int(d.Seconds())) + " s"
}

func (p *playsPage) flashSelected() {
	pl := p.current()
	if pl == nil || !(pl.Ready || p.model.ready[pl.Slug] || p.a.c.FirmwareReady(pl.Slug)) {
		return
	}
	runFlashDialog(p.a, pl)
}

func (p *playsPage) fetchSelected() {
	if pl := p.current(); pl != nil {
		p.a.showError(p.a.c.FetchFirmware(pl.Slug))
	}
}

func (p *playsPage) toggleSync() {
	if p.a.c.Sync.Running() {
		if p.a.c.Sync.Snapshot().Kind == "sync" {
			p.a.c.Sync.Cancel()
		}
		return
	}
	p.a.showError(p.a.c.StartSync())
}

func (p *playsPage) chooseFirstFolder() {
	if dir, ok := chooseFolder(p.a.mw, t("choose_library"), p.folder.Text()); ok {
		setText(p.folder, dir)
	}
}

// firstSync saves the chosen folder and starts the first download.
func (p *playsPage) firstSync() {
	c := p.a.c
	if dir := p.folder.Text(); dir != c.Settings.Get().LibraryDir {
		if err := c.SetLibrary(dir, "switch"); err != nil {
			p.a.showError(err)
			return
		}
	}
	p.a.showError(c.StartSync())
}

// refresh updates the sync row, the update banner and ready markers.
func (p *playsPage) refresh() {
	c := p.a.c
	st := c.Sync.Snapshot()
	if v := c.ReadyVersion(); v != p.readyVer {
		p.readyVer = v
		slugs, _ := c.Sync.Ready()
		p.model.ready = map[string]bool{}
		for _, s := range slugs {
			p.model.ready[s] = true
		}
		if n := len(p.model.rows); n > 0 {
			p.model.PublishRowsChanged(0, n-1)
		}
	}

	running := st.Running
	setVisible(p.syncBox, running || st.Phase == "failed")
	if running && st.Kind == "sync" {
		setText(p.syncBtn, t("stop"))
	} else {
		setText(p.syncBtn, t("sync_title"))
	}
	setEnabled(p.syncBtn, !(running && st.Kind == "move") && !c.Flash.Active())
	if running || st.Phase == "failed" {
		text, info := syncLine(st)
		setText(p.syncText, text)
		setText(p.syncInfo, info)
		pct := 0
		if st.Total > 0 {
			pct = st.Done * 100 / st.Total
		}
		setProgress(p.syncBar, pct)
		if st.Phase == "failed" {
			p.syncText.SetTextColor(red)
		} else {
			p.syncText.SetTextColor(walk.RGB(0, 0, 0))
		}
	}
	key := st.Phase + st.FinishedAt.String()
	if !running && key != p.lastSync {
		p.lastSync = key
		if st.Phase == "done" || st.Phase == "cancelled" {
			p.reload()
		}
	}

	up := c.Sync.Update()
	setVisible(p.banner, up.Available && !running)
	if up.Available {
		setText(p.banner, i18n.BannerUpdate(up.Changes))
	}
	if pl := p.current(); pl != nil {
		p.refreshFirmware(pl)
	}
}

// syncLine describes the sync state: the phase and the counters.
func syncLine(st syncer.State) (string, string) {
	if st.Phase == "failed" {
		return t("sync_failed") + "：" + i18n.ErrText(st.ErrorCode, st.Error), ""
	}
	phase := t("phase_" + st.Phase)
	if !i18n.Has("phase_" + st.Phase) {
		phase = t("sync_running")
	}
	info := []string{}
	if st.Total > 0 {
		info = append(info, strconv.Itoa(st.Done)+" / "+strconv.Itoa(st.Total))
	}
	if st.FirmwareTotal > 0 && st.Phase != "moving" {
		info = append(info, i18n.FirmwareProgress(st.FirmwareDone, st.FirmwareTotal))
	}
	if st.Failed > 0 {
		info = append(info, t("failed_count")+" "+strconv.Itoa(st.Failed))
	}
	if !st.ThrottledUntil.IsZero() && time.Until(st.ThrottledUntil) > 0 {
		info = append(info, i18n.Throttled(wait(st.ThrottledUntil)))
	}
	return t("sync_phase") + phase, strings.Join(info, "    ")
}

// viewed counts a view once a play has stayed selected for a moment (so that
// scrolling through the list with the arrow keys does not count every row) and
// loads its live likes and favorites.
func (p *playsPage) viewed(pl *library.Play) {
	if p.viewTimer != nil {
		p.viewTimer.Stop()
	}
	slug := pl.Slug
	p.viewTimer = time.AfterFunc(800*time.Millisecond, func() {
		p.a.mw.Synchronize(func() {
			if cur := p.current(); cur != nil && cur.Slug == slug {
				p.a.recordView(cur)
				p.a.loadEngagement(cur)
			}
		})
	})
}

// refreshEngagement shows the like, favorite and comment counts of the selected play.
func (p *playsPage) refreshEngagement() {
	pl := p.current()
	if pl == nil || p.heartBtn == nil {
		return
	}
	e := p.a.engagementOf(pl)
	heart, fav := "♡ "+t("like"), "☆ "+t("fav")
	if e.liked {
		heart = "♥ " + t("liked")
	}
	if e.saved {
		fav = "★ " + t("faved")
	}
	setText(p.heartBtn, heart+" "+strconv.Itoa(e.hearts))
	setText(p.favBtn, fav+" "+strconv.Itoa(e.favorites))
	setText(p.commentBtn, t("tab_comments")+" "+strconv.Itoa(pl.Comments))
}

// reloadEngagement fetches the selected play's counts again (after signing in).
func (p *playsPage) reloadEngagement() {
	if pl := p.current(); pl != nil {
		p.a.loadEngagement(pl)
	}
	p.refreshEngagement()
}

// selectSlug selects a play in the table, clearing filters that hide it.
func (p *playsPage) selectSlug(slug string) {
	for pass := 0; pass < 2; pass++ {
		for i, r := range p.model.rows {
			if r.Slug == slug {
				_ = p.table.SetCurrentIndex(i)
				p.table.EnsureItemVisible(i)
				return
			}
		}
		setText(p.search, "")
		_ = p.category.SetCurrentIndex(0)
		p.onlyReady.SetChecked(false)
		p.apply()
	}
}
