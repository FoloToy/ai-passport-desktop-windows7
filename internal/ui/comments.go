//go:build windows

package ui

import (
	"strconv"
	"strings"

	"github.com/FoloToy/ai-passport-desktop/pkg/library"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"github.com/FoloToy/ai-passport-desktop-windows7/internal/i18n"
)

// comment is one comment or reply (GET /api/plays/{slug}/comments).
type comment struct {
	ID         int       `json:"id"`
	AuthorName string    `json:"authorName"`
	IsAuthor   bool      `json:"isAuthor"`
	Status     string    `json:"status"`
	Rating     int       `json:"rating"`
	Body       string    `json:"body"`
	Likes      int       `json:"likes"`
	Liked      bool      `json:"liked"`
	CanDelete  bool      `json:"canDelete"`
	CreatedAt  string    `json:"createdAt"`
	Replies    []comment `json:"replies"`
}

func stars(n int) string {
	n = min(max(n, 0), 5)
	return strings.Repeat("★", n) + strings.Repeat("☆", 5-n)
}

type commentRow struct {
	c     *comment
	reply bool
}

type commentModel struct {
	walk.TableModelBase
	rows []commentRow
}

func (m *commentModel) RowCount() int { return len(m.rows) }

func (m *commentModel) Value(row, col int) interface{} {
	r := m.rows[row]
	c := r.c
	switch col {
	case 0:
		name := c.AuthorName
		if name == "" {
			name = "—"
		}
		if r.reply {
			name = "    ↳ " + name
		}
		if c.IsAuthor {
			name += "（" + t("c_author") + "）"
		}
		if c.Status == "pending" {
			name += "（" + t("c_pending") + "）"
		}
		return name
	case 1:
		if c.Rating > 0 {
			return stars(c.Rating)
		}
		return ""
	case 2:
		return strings.ReplaceAll(c.Body, "\n", " ")
	case 3:
		mark := "♡ "
		if c.Liked {
			mark = "♥ "
		}
		return mark + strconv.Itoa(c.Likes)
	case 4:
		return dateOf(c.CreatedAt)
	}
	return ""
}

// runComments shows a play's comments and lets a signed-in user post, reply,
// like and delete, as on the play page of the website and the Android app.
func runComments(a *app, pl *library.Play) {
	var dlg *walk.Dialog
	var table *walk.TableView
	var status, summary, replying *walk.Label
	var full *walk.TextEdit
	var likeBtn, replyBtn, delBtn, sendBtn, cancelReply *walk.PushButton
	var kind, rating *walk.ComboBox
	var body *walk.TextEdit
	var signedOut, composer *walk.Composite
	model := &commentModel{}
	var list []comment
	var replyTo *comment
	slug := pl.Slug

	current := func() *comment {
		i := table.CurrentIndex()
		if i < 0 || i >= len(model.rows) {
			return nil
		}
		return model.rows[i].c
	}
	update := func() {
		c := current()
		signed := a.user != nil
		setVisible(signedOut, !signed)
		setVisible(composer, signed)
		setEnabled(likeBtn, c != nil)
		setEnabled(replyBtn, c != nil && signed && !isReply(model.rows, table.CurrentIndex()))
		setVisible(delBtn, c != nil && c.CanDelete)
		if c != nil {
			setText(full, crlf(c.Body))
		} else {
			setText(full, "")
		}
		setVisible(replying, replyTo != nil)
		setVisible(cancelReply, replyTo != nil)
		if replyTo != nil {
			setText(replying, plain(i18n.Tf("c_replying_n", replyTo.AuthorName)))
		}
		setVisible(kind, replyTo == nil)
		setVisible(rating, replyTo == nil && kind.CurrentIndex() == 0)
	}
	fill := func(keep int) {
		model.rows = model.rows[:0]
		for i := range list {
			model.rows = append(model.rows, commentRow{c: &list[i]})
			for j := range list[i].Replies {
				model.rows = append(model.rows, commentRow{c: &list[i].Replies[j], reply: true})
			}
		}
		model.PublishRowsReset()
		if keep >= 0 {
			for i, r := range model.rows {
				if r.c.ID == keep {
					_ = table.SetCurrentIndex(i)
				}
			}
		}
		if len(list) == 0 {
			setText(status, t("comments_empty"))
		} else {
			setText(status, "")
		}
		update()
	}
	var load func()
	load = func() {
		setText(status, t("comments_loading"))
		var d struct {
			Comments []comment `json:"comments"`
			Summary  *struct {
				AverageRating float64 `json:"averageRating"`
				RatingCount   int     `json:"ratingCount"`
			} `json:"summary"`
		}
		a.async(func() error { return a.cm("GET", "/api/plays/"+slug+"/comments", "sort=default", nil, &d) }, func(err error) {
			if dlg == nil {
				return
			}
			if err != nil {
				if isStatus(err, 404) {
					list = nil
					fill(-1)
					return
				}
				setText(status, err.Error())
				return
			}
			list = d.Comments
			if s := d.Summary; s != nil && s.AverageRating > 0 {
				setText(summary, stars(int(s.AverageRating+0.5))+"  "+i18n.Tf("c_avg_n", strconv.FormatFloat(s.AverageRating, 'f', 1, 64), s.RatingCount))
			} else {
				setText(summary, "")
			}
			fill(-1)
		})
	}
	like := func() {
		c := current()
		if c == nil || !a.requireLogin() {
			return
		}
		method := "PUT"
		if c.Liked {
			method = "DELETE"
		}
		var d struct {
			Liked bool `json:"liked"`
			Likes *int `json:"likes"`
		}
		id := c.ID
		a.async(func() error {
			return a.cm(method, "/api/plays/"+slug+"/comments/"+strconv.Itoa(id)+"/like", "", nil, &d)
		}, func(err error) {
			if err != nil {
				a.showText(err.Error())
				return
			}
			c.Liked = d.Liked
			if d.Likes != nil {
				c.Likes = *d.Likes
			}
			fill(id)
		})
	}
	del := func() {
		c := current()
		if c == nil || walk.MsgBox(dlg, t("c_delete"), t("c_confirm_delete"), walk.MsgBoxOKCancel|walk.MsgBoxIconQuestion) != walk.DlgCmdOK {
			return
		}
		id := c.ID
		a.async(func() error {
			return a.cm("DELETE", "/api/plays/"+slug+"/comments/"+strconv.Itoa(id), "", nil, nil)
		}, func(err error) {
			if err != nil {
				a.showText(err.Error())
				return
			}
			load()
		})
	}
	send := func() {
		text := strings.TrimSpace(body.Text())
		if text == "" {
			walk.MsgBox(dlg, t("app_title"), t("c_need_body"), walk.MsgBoxIconWarning)
			return
		}
		req := map[string]any{"body": text, "kind": "suggestion"}
		if replyTo != nil {
			req["parentCommentId"] = replyTo.ID
		} else if kind.CurrentIndex() == 0 {
			req["kind"] = "review"
			n := rating.CurrentIndex() + 1
			if n < 1 {
				walk.MsgBox(dlg, t("app_title"), t("c_need_rating"), walk.MsgBoxIconWarning)
				return
			}
			req["rating"] = n
		}
		setEnabled(sendBtn, false)
		setText(sendBtn, t("c_sending"))
		var d struct {
			Comment *struct {
				Status string `json:"status"`
			} `json:"comment"`
		}
		a.async(func() error { return a.cm("POST", "/api/plays/"+slug+"/comments", "", req, &d) }, func(err error) {
			setEnabled(sendBtn, true)
			setText(sendBtn, t("c_send"))
			if err != nil {
				a.showText(err.Error())
				return
			}
			setText(body, "")
			replyTo = nil
			msg := t("c_posted")
			if d.Comment != nil && d.Comment.Status == "pending" {
				msg = t("c_pending_note")
			}
			walk.MsgBox(dlg, t("app_title"), msg, walk.MsgBoxIconInformation)
			load()
		})
	}

	err := Dialog{
		AssignTo: &dlg,
		Title:    t("tab_comments") + " - " + i18n.Text(pl.Title),
		Font:     uiFont(9, false),
		MinSize:  Size{Width: 760, Height: 560},
		Layout:   VBox{},
		Children: []Widget{
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				Label{Text: plain(i18n.Text(pl.Title)), Font: uiFont(12, true)},
				Label{AssignTo: &summary, TextColor: grey},
				HSpacer{},
				PushButton{Text: t("refresh"), OnClicked: func() { load() }},
			}},
			Label{AssignTo: &status, TextColor: grey},
			TableView{
				AssignTo:            &table,
				StretchFactor:       3,
				AlternatingRowBG:    true,
				LastColumnStretched: false,
				Columns: []TableViewColumn{
					{Title: t("col_author"), Width: 170},
					{Title: "", Width: 80},
					{Title: t("c_col_body"), Width: 300},
					{Title: "", Width: 60},
					{Title: "", Width: 90},
				},
				Model:                 model,
				OnCurrentIndexChanged: func() { update() },
			},
			TextEdit{AssignTo: &full, ReadOnly: true, VScroll: true, MinSize: Size{Height: 60}, StretchFactor: 1},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				PushButton{AssignTo: &likeBtn, Text: t("like"), OnClicked: like},
				PushButton{AssignTo: &replyBtn, Text: t("c_reply"), OnClicked: func() {
					if c := current(); c != nil {
						replyTo = c
						update()
						body.SetFocus()
					}
				}},
				PushButton{AssignTo: &delBtn, Text: t("c_delete"), Visible: false, OnClicked: del},
				HSpacer{},
			}},
			Composite{AssignTo: &signedOut, Layout: HBox{MarginsZero: true}, Children: []Widget{
				Label{Text: t("c_login_to_comment")},
				PushButton{Text: t("sign_in"), OnClicked: func() { runLoginDialog(a); update(); load() }},
				HSpacer{},
			}},
			Composite{AssignTo: &composer, Layout: VBox{MarginsZero: true}, Children: []Widget{
				Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
					ComboBox{AssignTo: &kind, Model: []string{t("c_review"), t("c_suggestion")}, CurrentIndex: 0, OnCurrentIndexChanged: func() { update() }},
					ComboBox{AssignTo: &rating, Model: []string{stars(1), stars(2), stars(3), stars(4), stars(5)}, CurrentIndex: 4},
					Label{AssignTo: &replying, Visible: false},
					PushButton{AssignTo: &cancelReply, Text: t("c_cancel"), Visible: false, OnClicked: func() { replyTo = nil; update() }},
					HSpacer{},
				}},
				TextEdit{AssignTo: &body, VScroll: true, MinSize: Size{Height: 60}, MaxLength: 2000},
				Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
					HSpacer{},
					PushButton{AssignTo: &sendBtn, Text: t("c_send"), OnClicked: send},
				}},
			}},
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
	update()
	load()
	a.runDialog(dlg)
	dlg = nil
}

func isReply(rows []commentRow, i int) bool { return i >= 0 && i < len(rows) && rows[i].reply }
