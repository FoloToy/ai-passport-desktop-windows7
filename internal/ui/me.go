//go:build windows

package ui

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/FoloToy/ai-passport-desktop/pkg/library"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"github.com/FoloToy/ai-passport-desktop-windows7/internal/i18n"
)

// mePage is 我的, laid out like the Android app: the account, the member lists,
// links, and the settings (library, language, about).
type mePage struct {
	a   *app
	tab int

	name      *walk.Label
	sub       *walk.Label
	signIn    *walk.PushButton
	profile   *walk.PushButton
	signOut   *walk.PushButton
	lists     *walk.GroupBox
	listLead  *walk.TextLabel
	messages  *walk.PushButton
	accountLk *walk.LinkLabel
}

var meLists = []string{"messages", "mine", "fav", "likes", "history"}

func (p *mePage) page() TabPage {
	listBtn := func(kind string, assign **walk.PushButton) Widget {
		return PushButton{AssignTo: assign, Text: t("me_" + kind), MinSize: Size{Width: 220}, OnClicked: func() { p.openList(kind) }}
	}
	var mine, fav, likes, history *walk.PushButton
	return TabPage{
		Title:  t("nav_me"),
		Layout: HBox{Alignment: AlignHNearVNear},
		Children: []Widget{
			Composite{
				StretchFactor: 1,
				Layout:        VBox{MarginsZero: true},
				Children: []Widget{
					GroupBox{
						Title:  t("me_title"),
						Layout: VBox{},
						Children: []Widget{
							Label{AssignTo: &p.name, Font: uiFont(13, true)},
							Label{AssignTo: &p.sub, TextColor: grey},
							Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
								PushButton{AssignTo: &p.signIn, Text: t("sign_in"), OnClicked: func() { runLoginDialog(p.a) }},
								PushButton{AssignTo: &p.profile, Text: t("me_profile") + " ↗", OnClicked: func() { p.a.openSite("account") }},
								PushButton{AssignTo: &p.signOut, Text: t("sign_out"), OnClicked: p.confirmSignOut},
								HSpacer{},
							}},
						},
					},
					GroupBox{
						AssignTo: &p.lists,
						Title:    t("me_lists"),
						Layout:   VBox{},
						Children: []Widget{
							TextLabel{AssignTo: &p.listLead, Text: t("me_signin_lead"), TextColor: grey, MinSize: Size{Width: 300}},
							listBtn("messages", &p.messages),
							listBtn("mine", &mine),
							listBtn("fav", &fav),
							listBtn("likes", &likes),
							listBtn("history", &history),
						},
					},
					GroupBox{
						Title:     t("me_more"),
						Layout:    VBox{},
						Alignment: AlignHNearVNear,
						Children: []Widget{
							LinkLabel{
								AssignTo: &p.accountLk,
								Text:     `<a id="account">` + t("me_account") + `</a>    <a id="terms">` + t("terms_title") + `</a>    <a id="privacy">` + t("privacy_title") + `</a>    <a id="creator">` + t("creator_center") + `</a>`,
								OnLinkActivated: func(l *walk.LinkLabelLink) {
									switch id := l.Id(); id {
									case "terms", "privacy":
										p.a.openSite(legal(id))
									default:
										p.a.openSite(id)
									}
								},
							},
						},
					},
					VSpacer{},
				},
			},
			Composite{
				StretchFactor: 1,
				Layout:        VBox{MarginsZero: true},
				Children:      p.a.settings.widgets(),
			},
		},
	}
}

func (p *mePage) init() { p.refreshAccount() }

// refreshAccount shows the signed-in user (or the sign-in button).
func (p *mePage) refreshAccount() {
	u := p.a.user
	switch {
	case !p.a.userKnown:
		setText(p.name, t("me_loading"))
		setText(p.sub, "")
	case u == nil:
		setText(p.name, t("me_signin_title"))
		setText(p.sub, t("login_same"))
	default:
		setText(p.name, plain(u.display()))
		sub := []string{}
		if u.MaskedPhone != "" {
			sub = append(sub, u.MaskedPhone)
		} else if u.Email != "" {
			sub = append(sub, u.Email)
		}
		if u.Followers != nil {
			sub = append(sub, i18n.Tf("followers_n", *u.Followers))
		}
		setText(p.sub, plain(strings.Join(sub, " · ")))
	}
	setVisible(p.signIn, p.a.userKnown && u == nil)
	setVisible(p.profile, u != nil)
	setVisible(p.signOut, u != nil)
	setVisible(p.listLead, u == nil)
	msg := t("me_messages")
	if u != nil && p.a.unread > 0 {
		msg += "（" + strconv.Itoa(p.a.unread) + "）"
	}
	setText(p.messages, msg)
	if u != nil && !p.a.unreadAsked {
		p.a.unreadAsked = true
		p.loadUnread()
	}
	if u == nil {
		p.a.unreadAsked = false
	}
}

func (p *mePage) loadUnread() {
	var d struct {
		UnreadCount   int               `json:"unreadCount"`
		Notifications []json.RawMessage `json:"notifications"`
	}
	p.a.async(func() error {
		return p.a.cm("GET", "/api/me/notifications", "category=all&status=unread&limit=30", nil, &d)
	}, func(err error) {
		if err != nil {
			return
		}
		p.a.unread = d.UnreadCount
		if p.a.unread == 0 {
			p.a.unread = len(d.Notifications)
		}
		p.refreshAccount()
	})
}

func (p *mePage) confirmSignOut() {
	if walk.MsgBox(p.a.mw, t("sign_out"), t("sign_out")+"？", walk.MsgBoxOKCancel|walk.MsgBoxIconQuestion) == walk.DlgCmdOK {
		p.a.signOut()
	}
}

func (p *mePage) openList(kind string) {
	if !p.a.requireLogin() {
		return
	}
	runMeList(p.a, kind)
	p.refreshAccount()
}

/* ---------------- the member lists ---------------- */

// meItem is one row of a member list, decoded loosely: the site's lists share
// most field names, and titles may be text or {zh, en}.
type meItem struct {
	ID          json.Number     `json:"id"`
	Slug        string          `json:"slug"`
	Title       json.RawMessage `json:"title"`
	Body        json.RawMessage `json:"body"`
	Status      string          `json:"status"`
	ReadAt      *string         `json:"readAt"`
	CreatedAt   string          `json:"createdAt"`
	CreatedAt2  string          `json:"created_at"`
	UpdatedAt   string          `json:"updatedAt"`
	InstalledAt string          `json:"installedAt"`
	HasUpdate   bool            `json:"hasUpdate"`
	Available   *bool           `json:"available"`
	Play        *struct {
		Slug  string          `json:"slug"`
		Title json.RawMessage `json:"title"`
	} `json:"play"`
}

// textOf reads a plain string or a {zh, en} object.
func textOf(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var tx library.Text
	if json.Unmarshal(raw, &tx) == nil {
		return i18n.Text(tx)
	}
	return ""
}

func dateOf(s string) string {
	if s == "" {
		return ""
	}
	if tm, err := time.Parse(time.RFC3339, s); err == nil {
		return tm.Local().Format("2006-01-02")
	}
	return day(s)
}

var submissionStatus = map[string][2]string{
	"pending": {"审核中", "In review"}, "approved": {"已通过", "Approved"}, "published": {"已发布", "Published"},
	"rejected": {"未通过", "Not approved"}, "draft": {"草稿", "Draft"}, "withdrawn": {"已撤回", "Withdrawn"},
}

type meListModel struct {
	walk.TableModelBase
	rows [][3]string
}

func (m *meListModel) RowCount() int                  { return len(m.rows) }
func (m *meListModel) Value(row, col int) interface{} { return m.rows[row][col] }

func runMeList(a *app, kind string) {
	var dlg *walk.Dialog
	var table *walk.TableView
	var status *walk.Label
	var readAll *walk.PushButton
	var body *walk.TextEdit
	model := &meListModel{}
	var items []meItem
	var local []*library.Play

	idx, _, _ := a.c.Index()
	find := func(it meItem) *library.Play {
		if idx == nil {
			return nil
		}
		slug := it.Slug
		if slug == "" && it.Play != nil {
			slug = it.Play.Slug
		}
		id, _ := it.ID.Int64()
		for i := range idx.Plays {
			pl := &idx.Plays[i]
			if (slug != "" && pl.Slug == slug) || (slug == "" && kind != "messages" && kind != "mine" && id > 0 && int64(pl.ID) == id) {
				return pl
			}
		}
		return nil
	}
	show := func() {
		model.rows = model.rows[:0]
		local = local[:0]
		unread := false
		for _, it := range items {
			var row [3]string
			var pl *library.Play
			switch kind {
			case "messages":
				row[0] = textOf(it.Title)
				if row[0] == "" {
					row[0] = t("msg_system")
				}
				if it.ReadAt == nil {
					row[0] = "● " + row[0]
					unread = true
				}
				row[1] = dateOf(it.CreatedAt)
				row[2] = textOf(it.Body)
			case "mine":
				row[0] = textOf(it.Title)
				if row[0] == "" {
					row[0] = it.Slug
				}
				st := submissionStatus[it.Status]
				row[1] = st[0]
				if i18n.Current() == "en" {
					row[1] = st[1]
				}
				if row[1] == "" {
					row[1] = it.Status
				}
				row[2] = dateOf(it.UpdatedAt)
				if row[2] == "" {
					row[2] = dateOf(it.CreatedAt)
				}
			default:
				pl = find(it)
				switch {
				case pl != nil:
					row[0] = i18n.Text(pl.Title)
				case it.Play != nil:
					row[0] = textOf(it.Play.Title)
				default:
					row[0] = textOf(it.Title)
				}
				if row[0] == "" {
					row[0] = "—"
				}
				if pl == nil || (it.Available != nil && !*it.Available) {
					row[1] = t("me_unavailable")
				}
				if kind == "history" {
					if it.HasUpdate {
						row[2] = t("has_update")
					} else {
						row[2] = dateOf(it.InstalledAt)
					}
				} else {
					row[2] = dateOf(it.CreatedAt)
					if row[2] == "" {
						row[2] = dateOf(it.CreatedAt2)
					}
				}
			}
			model.rows = append(model.rows, row)
			local = append(local, pl)
		}
		model.PublishRowsReset()
		setVisible(readAll, kind == "messages" && unread)
		if len(items) == 0 {
			setText(status, t("me_empty"))
		} else {
			setText(status, "")
		}
	}
	load := func() {
		setText(status, t("loading"))
		path := map[string][2]string{
			"messages": {"/api/me/notifications", "category=all&status=all&limit=30"},
			"mine":     {"/api/me/submissions", ""},
			"fav":      {"/api/me/follows", "kind=play&page=1"},
			"likes":    {"/api/me/likes", "page=1"},
			"history":  {"/api/me/install-history", "page=1"},
		}[kind]
		var d struct {
			Items         []meItem `json:"items"`
			Notifications []meItem `json:"notifications"`
			Submissions   []meItem `json:"submissions"`
			History       []meItem `json:"history"`
		}
		a.async(func() error { return a.cm("GET", path[0], path[1], nil, &d) }, func(err error) {
			if dlg == nil {
				return
			}
			if err != nil {
				setText(status, err.Error())
				return
			}
			for _, l := range [][]meItem{d.Items, d.Notifications, d.Submissions, d.History} {
				if len(l) > 0 {
					items = l
					break
				}
			}
			show()
		})
	}
	selected := func() {
		i := table.CurrentIndex()
		if kind != "messages" || i < 0 || i >= len(items) {
			setText(body, "")
			return
		}
		it := &items[i]
		setText(body, crlf(textOf(it.Body)))
		if it.ReadAt == nil {
			now := time.Now().UTC().Format(time.RFC3339)
			it.ReadAt = &now
			a.unread = max(0, a.unread-1)
			id := it.ID.String()
			go func() { _ = a.cm("PATCH", "/api/me/notifications/"+id+"/read", "", nil, nil) }()
			show()
			_ = table.SetCurrentIndex(i)
		}
	}
	activated := func() {
		i := table.CurrentIndex()
		if i < 0 || i >= len(local) || local[i] == nil {
			return
		}
		pl := local[i]
		dlg.Accept()
		a.showPlay(pl.Slug)
	}
	cols := []TableViewColumn{{Title: t("me_col_title"), Width: 300}, {Title: "", Width: 110}, {Title: "", Width: 110}}
	if kind == "messages" {
		cols = []TableViewColumn{{Title: t("me_col_title"), Width: 300}, {Title: t("me_col_date"), Width: 100}}
	}
	note := ""
	switch kind {
	case "mine":
		note = t("mine_note")
	case "fav", "likes", "history":
		note = t("me_open_hint")
	}
	err := Dialog{
		AssignTo: &dlg,
		Title:    t("me_" + kind),
		Font:     uiFont(9, false),
		MinSize:  Size{Width: 640, Height: 480},
		Layout:   VBox{},
		Children: []Widget{
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				Label{Text: t("me_" + kind), Font: uiFont(12, true)},
				HSpacer{},
				PushButton{AssignTo: &readAll, Text: t("msg_read_all"), Visible: false, OnClicked: func() {
					a.async(func() error { return a.cm("PATCH", "/api/me/notifications/read-all", "category=all", nil, nil) }, func(err error) {
						if err != nil {
							a.showText(err.Error())
							return
						}
						now := time.Now().UTC().Format(time.RFC3339)
						for i := range items {
							if items[i].ReadAt == nil {
								items[i].ReadAt = &now
							}
						}
						a.unread = 0
						show()
					})
				}},
				PushButton{Text: t("refresh"), OnClicked: load},
			}},
			TextLabel{Text: note, TextColor: grey, Visible: note != "", MinSize: Size{Width: 400}},
			Composite{Layout: HBox{MarginsZero: true}, Visible: kind == "mine", Children: []Widget{
				LinkLabel{Text: `<a>` + t("creator_center") + ` ↗</a>`, OnLinkActivated: func(*walk.LinkLabelLink) { a.openSite("creator") }},
				HSpacer{},
			}},
			Label{AssignTo: &status, TextColor: grey},
			TableView{
				AssignTo:              &table,
				StretchFactor:         2,
				AlternatingRowBG:      true,
				LastColumnStretched:   true,
				Columns:               cols,
				Model:                 model,
				OnCurrentIndexChanged: selected,
				OnItemActivated:       activated,
			},
			TextEdit{AssignTo: &body, ReadOnly: true, VScroll: true, Visible: kind == "messages", StretchFactor: 1},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				HSpacer{},
				PushButton{Text: t("close"), OnClicked: func() { dlg.Cancel() }},
			}},
		},
	}.Create(a.mw)
	if err != nil {
		a.showError(err)
		return
	}
	if icon := a.mw.Icon(); icon != nil {
		_ = dlg.SetIcon(icon)
	}
	load()
	a.runDialog(dlg)
	dlg = nil
}
