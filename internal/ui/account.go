//go:build windows

package ui

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/FoloToy/ai-passport-desktop/pkg/library"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"github.com/FoloToy/ai-passport-desktop-windows7/internal/i18n"
)

// user is the signed-in community account (GET /api/me).
type user struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	MaskedPhone string `json:"maskedPhone"`
	Followers   *int   `json:"followers"`
}

func (u *user) display() string {
	if u == nil {
		return ""
	}
	for _, s := range []string{u.Name, u.Email, u.MaskedPhone} {
		if s != "" {
			return s
		}
	}
	return ""
}

// siteError is a refusal from the community site, with its message.
type siteError struct {
	status int
	msg    string
}

func (e *siteError) Error() string { return e.msg }

// cm sends one request to the community site through pkg/account (the same
// allowlist, session and CSRF handling as the desktop app) and decodes the JSON
// answer into out. It blocks; call it off the UI thread.
func (a *app) cm(method, path, query string, body any, out any) error {
	if a.acct == nil {
		return errors.New(t("err_network"))
	}
	var raw []byte
	if body != nil {
		var err error
		if raw, err = json.Marshal(body); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	resp, err := a.acct.Do(ctx, method, path, query, raw)
	if err != nil {
		return errors.New(t("err_network"))
	}
	if resp.Status >= 400 {
		var d struct {
			Detail  any `json:"detail"`
			Error   any `json:"error"`
			Message any `json:"message"`
		}
		_ = json.Unmarshal(resp.Body, &d)
		msg := t("err_other")
		if resp.Status == 429 {
			msg = t("err_rate_limited")
		}
		for _, v := range []any{d.Detail, d.Error, d.Message} {
			if s, ok := v.(string); ok && s != "" {
				msg = s
				break
			}
		}
		return &siteError{status: resp.Status, msg: msg}
	}
	if out != nil && len(resp.Body) > 0 {
		return json.Unmarshal(resp.Body, out)
	}
	return nil
}

// async runs work in the background and then done on the UI thread.
func (a *app) async(work func() error, done func(error)) {
	go func() {
		err := work()
		a.mw.Synchronize(func() { done(err) })
	}()
}

// refreshMe asks the site who is signed in.
func (a *app) refreshMe() {
	var d struct {
		User *user `json:"user"`
	}
	a.async(func() error { return a.cm("GET", "/api/me", "", nil, &d) }, func(err error) {
		if err == nil || isStatus(err, 401) {
			a.user = d.User
		}
		a.userKnown = true
		a.me.refreshAccount()
	})
}

func isStatus(err error, status int) bool {
	var se *siteError
	return errors.As(err, &se) && se.status == status
}

// requireLogin opens the sign-in dialog when nobody is signed in.
func (a *app) requireLogin() bool {
	if a.user != nil {
		return true
	}
	walk.MsgBox(a.mw, t("app_title"), t("need_login"), walk.MsgBoxIconInformation)
	runLoginDialog(a)
	return a.user != nil
}

func (a *app) signOut() {
	a.async(func() error { return a.cm("POST", "/api/auth/logout", "", map[string]any{}, nil) }, func(error) {
		a.acct.SignOut()
		a.user = nil
		a.unread = 0
		a.eng = map[string]*engagement{}
		a.me.refreshAccount()
		a.plays.refreshEngagement()
	})
}

// openSite opens one of the community pages the app links to.
func (a *app) openSite(target string) {
	pages := map[string]string{
		"terms": "/legal/terms/", "privacy": "/legal/privacy/",
		"terms-en": "/en/legal/terms/", "privacy-en": "/en/legal/privacy/",
		"account": "/account/", "creator": "/creators/",
	}
	if p, ok := pages[target]; ok {
		openURL(strings.TrimRight(a.c.Sync.Client.Origin, "/") + p)
	}
}

func legal(name string) string {
	if i18n.Current() == "en" {
		return name + "-en"
	}
	return name
}

/* ---------------- statistics, as on the website and in the Android app ---------------- */

func playKind(p *library.Play) string {
	if p.Source == "official" || p.Official {
		return "official"
	}
	return "community"
}

// recordView counts a view of a play's details; the site ignores repeats.
func (a *app) recordView(p *library.Play) {
	kind, slug := playKind(p), p.Slug
	go func() { _ = a.cm("POST", "/api/engagement/"+kind+"/"+slug+"/view", "", map[string]any{}, nil) }()
}

// recordInstall adds a finished install to the signed-in user's history.
func (a *app) recordInstall(p *library.Play) {
	if a.user == nil || p == nil || p.ID == 0 {
		return
	}
	body := map[string]any{"revisionId": p.RevisionID, "firmwareSha256": p.Firmware.SHA256}
	go func() { _ = a.cm("POST", "/api/me/install-history/"+strconv.Itoa(p.ID), "", body, nil) }()
}

// engagement is the live like and favorite state of one play.
type engagement struct {
	hearts, favorites int
	liked, saved      bool
	busy              bool
}

func (a *app) engagementOf(p *library.Play) *engagement {
	e := a.eng[p.Slug]
	if e == nil {
		e = &engagement{hearts: p.Hearts, favorites: p.Favorites}
		a.eng[p.Slug] = e
	}
	return e
}

// loadEngagement fetches the live counts and the user's own like and favorite.
func (a *app) loadEngagement(p *library.Play) {
	e := a.engagementOf(p)
	target := playKind(p) + ":" + p.Slug
	signedIn := a.user != nil
	id := p.ID
	var hearts, favorites *int
	var liked, saved bool
	a.async(func() error {
		var d struct {
			Engagement map[string]struct {
				Hearts int  `json:"hearts"`
				Liked  bool `json:"liked"`
			} `json:"engagement"`
		}
		if err := a.cm("GET", "/api/engagement", "targets="+url.QueryEscape(target), nil, &d); err != nil {
			return err
		}
		if x, ok := d.Engagement[target]; ok {
			h := x.Hearts
			hearts, liked = &h, x.Liked
		}
		if signedIn && id > 0 {
			var f struct {
				Saved bool `json:"saved"`
				Count *int `json:"count"`
			}
			if err := a.cm("GET", "/api/me/follows/play/"+strconv.Itoa(id), "", nil, &f); err == nil {
				saved, favorites = f.Saved, f.Count
			}
		}
		return nil
	}, func(err error) {
		if err != nil {
			return
		}
		if hearts != nil {
			e.hearts = *hearts
		}
		if favorites != nil {
			e.favorites = *favorites
		}
		e.liked, e.saved = liked, saved
		a.plays.refreshEngagement()
	})
}

func (a *app) toggleHeart(p *library.Play) {
	e := a.engagementOf(p)
	if e.busy {
		return
	}
	e.busy = true
	method := "PUT"
	if e.liked {
		method = "DELETE"
	}
	var d struct {
		Liked  bool `json:"liked"`
		Hearts *int `json:"hearts"`
	}
	a.async(func() error {
		return a.cm(method, "/api/engagement/"+playKind(p)+"/"+p.Slug+"/heart", "", nil, &d)
	}, func(err error) {
		e.busy = false
		if err != nil {
			a.showText(err.Error())
			return
		}
		e.liked = d.Liked
		if d.Hearts != nil {
			e.hearts = *d.Hearts
		}
		a.plays.refreshEngagement()
	})
}

func (a *app) toggleFavorite(p *library.Play) {
	if !a.requireLogin() {
		return
	}
	e := a.engagementOf(p)
	if e.busy || p.ID == 0 {
		return
	}
	e.busy = true
	on := !e.saved
	method := "PUT"
	if !on {
		method = "DELETE"
	}
	var d struct {
		Saved *bool `json:"saved"`
		Count *int  `json:"count"`
	}
	a.async(func() error {
		return a.cm(method, "/api/me/follows/play/"+strconv.Itoa(p.ID), "", nil, &d)
	}, func(err error) {
		e.busy = false
		if err != nil {
			a.showText(err.Error())
			return
		}
		e.saved = on
		if d.Saved != nil {
			e.saved = *d.Saved
		}
		switch {
		case d.Count != nil:
			e.favorites = *d.Count
		case e.saved:
			e.favorites++
		case e.favorites > 0:
			e.favorites--
		}
		a.plays.refreshEngagement()
	})
}

/* ---------------- sign-in dialog ----------------
   Same as the Android app and the website: one screen, code sign-in by mobile
   number (with a country code) or email; a new account is created on first sign-in. */

var (
	reCNMobile = regexp.MustCompile(`^\+861[3-9]\d{9}$`)
	reCNLocal  = regexp.MustCompile(`^(?:86)?1[3-9]\d{9}$`)
	reE164     = regexp.MustCompile(`^\+[1-9]\d{6,14}$`)
	rePhoneIn  = regexp.MustCompile(`^[+0-9\s().-]{1,80}$`)
	reEmail    = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	reCode     = regexp.MustCompile(`^\d{6}$`)
)

func countryOf(iso string) [5]string {
	for _, c := range phoneCountries {
		if c[0] == iso {
			return c
		}
	}
	return phoneCountries[0]
}

// normalizePhone returns the number in E.164 or "" when it cannot be valid. A
// number typed with its own "+" or "00" prefix wins over the picked country.
func normalizePhone(value, iso string) string {
	if !rePhoneIn.MatchString(value) {
		return ""
	}
	s := strings.NewReplacer(" ", "", "\t", "", "(", "", ")", "", ".", "", "-", "").Replace(value)
	if strings.HasPrefix(s, "00") {
		s = "+" + s[2:]
	}
	if reCNMobile.MatchString(s) {
		return s
	}
	if iso == "CN" && reCNLocal.MatchString(s) {
		return "+86" + s[len(s)-11:]
	}
	if strings.HasPrefix(s, "+") {
		if strings.HasPrefix(s, "+86") || !reE164.MatchString(s) {
			return ""
		}
		return s
	}
	if iso == "CN" {
		return ""
	}
	digits := strings.TrimLeft(s, "0")
	full := "+" + countryOf(iso)[1] + digits
	if reE164.MatchString(full) && len(digits) >= 4 {
		return full
	}
	return ""
}

func countryName(c [5]string) string {
	if i18n.Current() == "en" {
		return c[3]
	}
	return c[2]
}

// accents folds the accented letters that occur in the English country names.
var accents = strings.NewReplacer("å", "a", "Å", "a", "ô", "o", "é", "e", "ü", "u", "ç", "c", "ã", "a", "í", "i", "ñ", "n")

func fold(s string) string { return strings.ToLower(accents.Replace(s)) }

// countryEntries is the picker order: the pinned four, then by pinyin (Chinese)
// or by English name, each with its index letter.
func countryEntries() (list [][5]string, groups []string) {
	pinned, rest := phoneCountries[:4], append([][5]string(nil), phoneCountries[4:]...)
	if i18n.Current() == "en" {
		sort.SliceStable(rest, func(i, j int) bool { return fold(rest[i][3]) < fold(rest[j][3]) })
	}
	for _, c := range pinned {
		list, groups = append(list, c), append(groups, "")
	}
	for _, c := range rest {
		g := c[4]
		if i18n.Current() == "en" {
			g = strings.ToUpper(fold(c[3])[:1])
		}
		list, groups = append(list, c), append(groups, g)
	}
	return list, groups
}

type loginDialog struct {
	a       *app
	dlg     *walk.Dialog
	channel string // "phone" or "email"
	iso     string

	byPhone, byEmail *walk.RadioButton
	targetLbl        *walk.Label
	ccBtn            *walk.PushButton
	target           *walk.LineEdit
	codeLbl          *walk.Label
	code             *walk.LineEdit
	sendBtn          *walk.PushButton
	errLbl           *walk.TextLabel
	loginBtn         *walk.PushButton
	terms            *walk.CheckBox

	cooldown int
	busy     bool
	ticker   *time.Ticker
}

func runLoginDialog(a *app) {
	d := &loginDialog{a: a, channel: "phone", iso: "CN"}
	if a.loginIso != "" {
		d.iso = a.loginIso
	}
	tl, pl := legal("terms"), legal("privacy")
	wrap := Size{Width: 380}
	err := Dialog{
		AssignTo:      &d.dlg,
		Title:         t("sign_in"),
		Font:          uiFont(9, false),
		MinSize:       Size{Width: 440, Height: 420},
		Layout:        VBox{Margins: Margins{Left: 18, Top: 14, Right: 18, Bottom: 14}},
		DefaultButton: &d.loginBtn,
		Children: []Widget{
			Label{Text: t("login_hero1") + t("login_hero2"), Font: uiFont(13, true)},
			TextLabel{Text: t("login_same"), TextColor: grey, MinSize: wrap},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				RadioButtonGroup{Buttons: []RadioButton{
					{AssignTo: &d.byEmail, Text: t("by_email"), OnClicked: func() { d.setChannel("email") }},
					{AssignTo: &d.byPhone, Text: t("by_phone"), OnClicked: func() { d.setChannel("phone") }},
				}},
				HSpacer{},
			}},
			Label{AssignTo: &d.targetLbl},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				PushButton{AssignTo: &d.ccBtn, MaxSize: Size{Width: 90}, OnClicked: d.pickCountry},
				LineEdit{AssignTo: &d.target, StretchFactor: 1},
			}},
			Label{AssignTo: &d.codeLbl},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				LineEdit{AssignTo: &d.code, CueBanner: t("ph_code"), MaxLength: 6, StretchFactor: 1},
				PushButton{AssignTo: &d.sendBtn, Text: t("send_code"), MinSize: Size{Width: 110}, OnClicked: d.sendCode},
			}},
			TextLabel{AssignTo: &d.errLbl, TextColor: red, MinSize: wrap},
			PushButton{AssignTo: &d.loginBtn, Text: t("submit_login"), MinSize: Size{Height: 32}, OnClicked: d.login},
			CheckBox{AssignTo: &d.terms, Alignment: AlignHNearVCenter, Text: plain(t("agree_pre") + t("terms") + t("and") + t("privacy") + t("agree_post"))},
			LinkLabel{
				Text: `<a id="t">` + t("terms") + `</a>    <a id="p">` + t("privacy") + `</a>`,
				OnLinkActivated: func(l *walk.LinkLabelLink) {
					map[string]func(){"t": func() { a.openSite(tl) }, "p": func() { a.openSite(pl) }}[l.Id()]()
				},
			},
			VSpacer{},
		},
	}.Create(a.mw)
	if err != nil {
		a.showError(err)
		return
	}
	if icon := a.mw.Icon(); icon != nil {
		_ = d.dlg.SetIcon(icon)
	}
	d.byPhone.SetChecked(true)
	d.setChannel("phone")
	d.ticker = time.NewTicker(time.Second)
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-d.ticker.C:
				d.dlg.Synchronize(d.tick)
			}
		}
	}()
	a.runDialog(d.dlg)
	d.ticker.Stop()
	close(stop)
	a.loginIso = d.iso
}

func (d *loginDialog) tick() {
	if d.cooldown > 0 {
		d.cooldown--
		d.refresh()
	}
}

func (d *loginDialog) setChannel(ch string) {
	if ch != d.channel {
		setText(d.target, "")
		setText(d.code, "")
	}
	d.channel = ch
	setText(d.errLbl, "")
	phone := ch == "phone"
	setVisible(d.ccBtn, phone)
	if phone {
		setText(d.targetLbl, t("ch_phone"))
		setText(d.codeLbl, t("sms_code"))
		_ = d.target.SetCueBanner(t("ph_phone"))
	} else {
		setText(d.targetLbl, t("ch_email"))
		setText(d.codeLbl, t("email_code"))
		_ = d.target.SetCueBanner("name@example.com")
	}
	d.refresh()
}

func (d *loginDialog) refresh() {
	setText(d.ccBtn, "+"+countryOf(d.iso)[1]+" ▾")
	switch {
	case d.cooldown > 0:
		setText(d.sendBtn, strconv.Itoa(d.cooldown)+"s")
	case d.busy:
		setText(d.sendBtn, t("sending"))
	default:
		setText(d.sendBtn, t("send_code"))
	}
	setEnabled(d.sendBtn, d.cooldown == 0 && !d.busy)
	setEnabled(d.loginBtn, !d.busy)
}

// targetBody is {"phone": E.164} or {"email": …}; nil when the input is invalid.
func (d *loginDialog) targetBody() map[string]any {
	v := strings.TrimSpace(d.target.Text())
	if d.channel == "phone" {
		if p := normalizePhone(v, d.iso); p != "" {
			return map[string]any{"phone": p}
		}
		return nil
	}
	if reEmail.MatchString(v) {
		return map[string]any{"email": v}
	}
	return nil
}

func (d *loginDialog) badTarget() string {
	if d.channel == "phone" {
		return t("bad_phone")
	}
	return t("bad_email")
}

func locale() string {
	if i18n.Current() == "en" {
		return "en"
	}
	return "zh"
}

func (d *loginDialog) sendCode() {
	body := d.targetBody()
	if body == nil {
		setText(d.errLbl, d.badTarget())
		return
	}
	body["channel"], body["locale"] = d.channel, locale()
	d.busy = true
	setText(d.errLbl, "")
	d.refresh()
	var r struct {
		Message string `json:"message"`
	}
	d.a.async(func() error {
		return d.a.cm("POST", "/api/auth/code-login/verification-code/request", "", body, &r)
	}, func(err error) {
		d.busy = false
		if err != nil {
			setText(d.errLbl, err.Error())
		} else {
			d.cooldown = 60
			msg := r.Message
			if msg == "" {
				msg = t("code_sent")
			}
			setText(d.errLbl, "")
			d.errLbl.SetTextColor(green)
			setText(d.errLbl, msg)
		}
		d.refresh()
	})
}

func (d *loginDialog) login() {
	d.errLbl.SetTextColor(red)
	body := d.targetBody()
	code := strings.TrimSpace(d.code.Text())
	switch {
	case body == nil:
		setText(d.errLbl, d.badTarget())
		return
	case !reCode.MatchString(code):
		setText(d.errLbl, t("bad_code"))
		return
	case !d.terms.Checked():
		setText(d.errLbl, t("must_agree"))
		return
	}
	body["channel"], body["verification_code"], body["terms_accepted"], body["locale"] = d.channel, code, true, locale()
	d.busy = true
	setText(d.errLbl, "")
	d.refresh()
	var r struct {
		User *user `json:"user"`
	}
	var me struct {
		User *user `json:"user"`
	}
	d.a.async(func() error {
		if err := d.a.cm("POST", "/api/auth/code-login", "", body, &r); err != nil {
			return err
		}
		_ = d.a.cm("GET", "/api/me", "", nil, &me)
		return nil
	}, func(err error) {
		d.busy = false
		if err != nil {
			setText(d.errLbl, err.Error())
			d.refresh()
			return
		}
		u := me.User
		if u == nil {
			u = r.User
		}
		d.a.user, d.a.userKnown = u, true
		d.a.eng = map[string]*engagement{}
		d.a.me.refreshAccount()
		d.a.plays.reloadEngagement()
		d.dlg.Accept()
		walk.MsgBox(d.a.mw, t("app_title"), i18n.Tf("welcome_back_n", u.display()), walk.MsgBoxIconInformation)
	})
}

// pickCountry is the "country or region code" list with a search box, as on Android.
func (d *loginDialog) pickCountry() {
	var dlg *walk.Dialog
	var search *walk.LineEdit
	var list *walk.ListBox
	all, groups := countryEntries()
	var shown [][5]string
	fill := func() {
		q := strings.TrimPrefix(fold(strings.TrimSpace(search.Text())), "+")
		shown = shown[:0]
		names := []string{}
		last := "-"
		for i, c := range all {
			if q != "" && !strings.Contains(fold(countryName(c)), q) && !strings.Contains(c[2], q) && strings.ToLower(c[0]) != q && !strings.HasPrefix(c[1], q) {
				continue
			}
			label := countryName(c) + "    +" + c[1]
			if q == "" && groups[i] != "" && groups[i] != last {
				label = groups[i] + "   " + label
			} else if q == "" && groups[i] != "" {
				label = "     " + label
			}
			last = groups[i]
			shown = append(shown, c)
			names = append(names, label)
		}
		_ = list.SetModel(names)
		for i, c := range shown {
			if c[0] == d.iso {
				_ = list.SetCurrentIndex(i)
			}
		}
	}
	pick := func() {
		if i := list.CurrentIndex(); i >= 0 && i < len(shown) {
			d.iso = shown[i][0]
			dlg.Accept()
		}
	}
	err := Dialog{
		AssignTo: &dlg,
		Title:    t("cc_title"),
		Font:     uiFont(9, false),
		MinSize:  Size{Width: 340, Height: 480},
		Layout:   VBox{},
		Children: []Widget{
			LineEdit{AssignTo: &search, CueBanner: t("cc_search"), OnTextChanged: func() { fill() }},
			ListBox{AssignTo: &list, StretchFactor: 1, OnItemActivated: pick},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				HSpacer{},
				PushButton{Text: t("ok"), OnClicked: pick},
				PushButton{Text: t("cancel"), OnClicked: func() { dlg.Cancel() }},
			}},
		},
	}.Create(d.dlg)
	if err != nil {
		return
	}
	fill()
	dlg.Run()
	d.refresh()
}
