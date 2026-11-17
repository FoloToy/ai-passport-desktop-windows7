//go:build windows

package ui

import (
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/FoloToy/ai-passport-desktop/pkg/core"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"github.com/FoloToy/ai-passport-desktop-windows7/internal/i18n"
)

// flashLocal installs a firmware file the user picks (工具 › 安装本地固件).
func (p *toolsPage) flashLocal() {
	dlg := &walk.FileDialog{Title: t("file_choose"), Filter: t("file_filter") + " (*.bin)|*.bin|" + t("file_all") + " (*.*)|*.*"}
	if ok, err := dlg.ShowOpen(p.a.mw); err != nil || !ok || dlg.FilePath == "" {
		return
	}
	path := dlg.FilePath
	openFlash(p.a, flashTarget{
		title: filepath.Base(path),
		note:  t("file_hint"),
		agree: t("flash_agree"),
		start: func(port string) error { return p.a.c.StartFlashFile(path, port) },
	})
}

// installInstaller installs the Play Installer, downloading its firmware first
// when needed; the install dialog opens by itself once the download is done.
func (p *toolsPage) installInstaller() {
	c := p.a.c
	pl, err := c.Play(core.InstallerSlug)
	if err != nil || pl == nil {
		walk.MsgBox(p.a.mw, t("t_installer"), t("installer_missing"), walk.MsgBoxIconWarning)
		return
	}
	if pl.Ready || c.FirmwareReady(pl.Slug) {
		p.instWanted = false
		runFlashDialog(p.a, pl)
		return
	}
	if err := c.FetchFirmware(pl.Slug); err != nil {
		p.a.showError(err)
		return
	}
	p.instWanted = true
}

// refreshInstaller follows the firmware download started by installInstaller.
func (p *toolsPage) refreshInstaller() {
	if !p.instWanted {
		setVisible(p.instStatus, false)
		return
	}
	c := p.a.c
	f := c.Sync.Fetch()
	setVisible(p.instStatus, true)
	switch {
	case c.FirmwareReady(core.InstallerSlug):
		p.instWanted = false
		setVisible(p.instStatus, false)
		if pl, err := c.Play(core.InstallerSlug); err == nil && pl != nil && p.a.flashDlg == nil {
			p.a.mw.Synchronize(func() { runFlashDialog(p.a, pl) })
		}
	case f.Slug != core.InstallerSlug:
		setText(p.instStatus, t("fetching"))
	case f.Running && !f.ThrottledUntil.IsZero():
		setText(p.instStatus, i18n.FetchWait(wait(f.ThrottledUntil)))
	case f.Running:
		setText(p.instStatus, t("fetching"))
	case f.ErrorCode != "" || f.Error != "":
		p.instWanted = false
		walk.MsgBox(p.a.mw, t("t_installer"), i18n.ErrText(f.ErrorCode, f.Error), walk.MsgBoxIconError)
	}
}

/* ---------------- 我的备份 ---------------- */

type backupModel struct {
	walk.TableModelBase
	rows []core.Backup
}

func (m *backupModel) RowCount() int { return len(m.rows) }

func (m *backupModel) Value(row, col int) interface{} {
	b := m.rows[row]
	switch col {
	case 0:
		return b.Time.Local().Format("2006-01-02 15:04")
	case 1:
		return t("backups_kind_" + b.Kind)
	case 2:
		return macColons(b.MAC)
	case 3:
		return i18n.Bytes(b.Size)
	}
	return ""
}

// macColons writes "aabbccddeeff" as "AA:BB:CC:DD:EE:FF".
func macColons(mac string) string {
	if len(mac) != 12 {
		return strings.ToUpper(mac)
	}
	parts := make([]string, 6)
	for i := range parts {
		parts[i] = mac[i*2 : i*2+2]
	}
	return strings.ToUpper(strings.Join(parts, ":"))
}

// runBackups lists the full-flash backups and writes one back to its own device.
func runBackups(a *app) {
	var dlg *walk.Dialog
	var table *walk.TableView
	var empty *walk.Label
	var writeBtn, showBtn, delBtn *walk.PushButton
	model := &backupModel{}
	current := func() *core.Backup {
		i := table.CurrentIndex()
		if i < 0 || i >= len(model.rows) {
			return nil
		}
		return &model.rows[i]
	}
	update := func() {
		b := current()
		for _, w := range []*walk.PushButton{writeBtn, showBtn, delBtn} {
			setEnabled(w, b != nil)
		}
	}
	load := func() {
		list, err := a.c.ListBackups()
		if err != nil {
			a.showError(err)
		}
		model.rows = list
		model.PublishRowsReset()
		setVisible(empty, len(list) == 0)
		update()
	}
	write := func() {
		b := current()
		if b == nil {
			return
		}
		bk := *b
		dlg.Accept()
		openFlash(a, flashTarget{
			title: t("backups_kind_"+bk.Kind) + " · " + macColons(bk.MAC),
			note:  i18n.Tf("backup_mac_n", macColons(bk.MAC)) + "\r\n" + t("backups_lead"),
			agree: t("flash_agree_backup"),
			start: func(port string) error { return a.c.StartRestoreBackup(bk.Path, port) },
		})
	}
	err := Dialog{
		AssignTo: &dlg,
		Title:    t("backups_title"),
		Font:     uiFont(9, false),
		MinSize:  Size{Width: 640, Height: 440},
		Layout:   VBox{},
		Children: []Widget{
			TextLabel{Text: t("backups_lead"), MinSize: Size{Width: 400}},
			Label{AssignTo: &empty, Text: t("backups_empty"), TextColor: grey, Visible: false},
			TableView{
				AssignTo:            &table,
				StretchFactor:       1,
				AlternatingRowBG:    true,
				LastColumnStretched: true,
				Columns: []TableViewColumn{
					{Title: t("me_col_date"), Width: 140},
					{Title: t("backups_col_kind"), Width: 180},
					{Title: "MAC", Width: 150},
					{Title: t("size"), Width: 80},
				},
				Model:                 model,
				OnCurrentIndexChanged: update,
				OnItemActivated:       write,
			},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				PushButton{AssignTo: &writeBtn, Text: t("b_write"), OnClicked: write},
				PushButton{AssignTo: &showBtn, Text: t("b_show"), OnClicked: func() {
					if b := current(); b != nil {
						cmd := exec.Command("explorer")
						cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer /select,"` + b.Path + `"`}
						_ = cmd.Start()
					}
				}},
				PushButton{AssignTo: &delBtn, Text: t("b_delete"), OnClicked: func() {
					b := current()
					if b == nil || walk.MsgBox(dlg, t("b_delete"), t("b_delete_q"), walk.MsgBoxOKCancel|walk.MsgBoxIconQuestion) != walk.DlgCmdOK {
						return
					}
					a.showError(a.c.DeleteBackup(b.Path))
					load()
				}},
				HSpacer{},
				PushButton{Text: t("backups_new"), OnClicked: func() {
					dlg.Accept()
					a.goTab(a.tools.tab)
					a.tools.backup()
				}},
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
}
