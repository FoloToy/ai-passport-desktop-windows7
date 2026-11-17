//go:build windows

package ui

import (
	"strings"
	"time"

	"github.com/FoloToy/ai-passport-desktop/pkg/factory"
	"github.com/FoloToy/ai-passport-desktop/pkg/flasher"
	"github.com/FoloToy/ai-passport-desktop/pkg/settings"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"github.com/FoloToy/ai-passport-desktop-windows7/internal/driver"
	"github.com/FoloToy/ai-passport-desktop-windows7/internal/i18n"
)

// waitMax is how long a restore or backup waits for the device (pkg/device default).
const waitMax = 150

// toolsPage is 工具, laid out like the Android app minus NFC: the default play,
// installing with a USB cable (local firmware, backup, my backups, factory
// restore), the Play Installer, and the Bluetooth items, which are shown as
// unavailable. The status of a backup or restore is on the right.
type toolsPage struct {
	a   *app
	tab int

	backupBtn  *walk.PushButton
	restoreBtn *walk.PushButton
	stopBtn    *walk.PushButton
	fullBackup *walk.CheckBox
	backupDir  *walk.LineEdit
	localBtn   *walk.PushButton
	backupsBtn *walk.PushButton
	backBtn    *walk.PushButton
	instBtn    *walk.PushButton
	instStatus *walk.TextLabel
	instWanted bool

	stage   *walk.Label
	sub     *walk.TextLabel
	hint    *walk.TextLabel
	bar     *walk.ProgressBar
	saved   *walk.TextLabel
	openBtn *walk.PushButton
	logView *walk.TextEdit
	logs    int

	driverText *walk.TextLabel
	driverBtn  *walk.PushButton
	driverBusy bool
	driverNext time.Time
	driverDone chan string
}

// toolRow is one entry of a section: a button with its explanation, or a
// greyed-out entry marked "needs Bluetooth".
func toolRow(assign **walk.PushButton, title, sub string, onClick walk.EventHandler) Widget {
	ble := onClick == nil
	if ble {
		title += "（" + t("needs_ble") + "）"
	}
	return Composite{Layout: HBox{MarginsZero: true, Alignment: AlignHNearVCenter}, Children: []Widget{
		PushButton{AssignTo: assign, Text: title, MinSize: Size{Width: 210}, MaxSize: Size{Width: 210}, Enabled: !ble, OnClicked: onClick},
		TextLabel{Text: sub, TextColor: grey, MinSize: Size{Width: 220}, StretchFactor: 1},
	}}
}

func (p *toolsPage) page() TabPage {
	wrap := Size{Width: 300}
	var noBtn *walk.PushButton
	return TabPage{
		Title:  t("nav_tools"),
		Layout: HBox{Alignment: AlignHNearVNear},
		Children: []Widget{
			ScrollView{
				StretchFactor:   3,
				HorizontalFixed: true,
				Layout:          VBox{MarginsZero: true},
				Children: []Widget{
					TextLabel{Text: t("tools_lead"), MinSize: wrap},
					GroupBox{
						Title:  t("sec_default") + "（" + t("pill_default") + "）",
						Layout: VBox{},
						Children: []Widget{
							TextLabel{Text: t("sec_default_info"), MinSize: wrap},
							toolRow(&noBtn, t("t_edit_passport"), t("t_edit_passport_sub"), nil),
							toolRow(&p.backBtn, t("t_passport_back"), t("t_passport_back_sub"), func() { p.restore() }),
						},
					},
					GroupBox{
						Title:  t("sec_usb"),
						Layout: VBox{},
						Children: []Widget{
							toolRow(&p.localBtn, t("t_local"), t("t_local_sub"), func() { p.flashLocal() }),
							toolRow(&p.backupBtn, t("t_backup"), t("t_backup_sub"), func() { p.backup() }),
							toolRow(&p.backupsBtn, t("t_backups"), t("t_backups_sub"), func() { runBackups(p.a) }),
							toolRow(&p.restoreBtn, t("t_restore"), t("t_restore_sub"), func() { p.restore() }),
							Composite{Layout: VBox{Margins: Margins{Left: 220}}, Children: []Widget{
								TextLabel{Text: "1. " + t("restore_step1") + "\r\n2. " + t("restore_step2"), MinSize: wrap},
								CheckBox{AssignTo: &p.fullBackup, Text: t("r_full"), Checked: true, Alignment: AlignHNearVCenter},
								TextLabel{Text: t("r_safety"), TextColor: grey, MinSize: wrap},
								Label{Text: t("r_fw") + "：" + factory.Version, TextColor: grey},
								Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
									PushButton{AssignTo: &p.stopBtn, Text: t("restore_stop"), Enabled: false, OnClicked: func() {
										p.a.showError(p.a.c.CancelRestore())
									}},
									HSpacer{},
								}},
							}},
						},
					},
					GroupBox{
						Title:  t("sec_installer"),
						Layout: VBox{},
						Children: []Widget{
							toolRow(&p.instBtn, t("t_installer"), t("t_installer_sub"), func() { p.installInstaller() }),
							TextLabel{AssignTo: &p.instStatus, TextColor: grey, MinSize: wrap, Visible: false},
							toolRow(&noBtn, t("t_installer_wifi"), t("t_installer_wifi_sub"), nil),
						},
					},
					GroupBox{
						Title:  t("sec_blufi"),
						Layout: VBox{},
						Children: []Widget{
							toolRow(&noBtn, t("t_blufi"), t("t_blufi_sub"), nil),
						},
					},
					GroupBox{
						Title:  t("r_backup_dir"),
						Layout: HBox{},
						Children: []Widget{
							LineEdit{AssignTo: &p.backupDir, ReadOnly: true},
							PushButton{Text: t("change"), OnClicked: p.chooseBackupDir},
							PushButton{Text: t("open_backup"), OnClicked: func() { reveal(p.a.c.Settings.Get().Backups()) }},
						},
					},
					GroupBox{
						Title:  t("driver_title"),
						Layout: VBox{},
						Children: []Widget{
							TextLabel{AssignTo: &p.driverText, MinSize: wrap},
							Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
								PushButton{AssignTo: &p.driverBtn, Text: t("driver_install"), OnClicked: p.installDriver},
								HSpacer{},
							}},
						},
					},
				},
			},
			Composite{
				StretchFactor: 2,
				Layout:        VBox{MarginsZero: true},
				Children: []Widget{GroupBox{
					Title:  t("status"),
					Layout: VBox{},
					Children: []Widget{
						Label{AssignTo: &p.stage, Font: uiFont(12, true)},
						TextLabel{AssignTo: &p.sub, MinSize: wrap},
						TextLabel{AssignTo: &p.hint, TextColor: red, MinSize: wrap},
						ProgressBar{AssignTo: &p.bar, MaxValue: 100},
						TextLabel{AssignTo: &p.saved, MinSize: wrap},
						Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
							PushButton{AssignTo: &p.openBtn, Text: t("open_backup"), Visible: false, OnClicked: func() {
								if dir, err := p.a.c.RevealPath("restore-session", ""); err == nil {
									reveal(dir)
								}
							}},
							HSpacer{},
						}},
						Label{Text: t("log"), TextColor: grey},
						TextEdit{AssignTo: &p.logView, ReadOnly: true, VScroll: true, MinSize: Size{Height: 200}},
					},
				}, VSpacer{}},
			},
		},
	}
}

func (p *toolsPage) init() {
	p.driverDone = make(chan string, 1)
	p.showBackupDir()
}

func (p *toolsPage) showBackupDir() {
	dir := p.a.c.Settings.Get().Backups()
	if dir == settings.DefaultBackupDir() {
		dir += "（" + t("default_folder") + "）"
	}
	setText(p.backupDir, dir)
}

func (p *toolsPage) backup() {
	p.logs = -1
	p.a.showError(p.a.c.StartBackup())
}

func (p *toolsPage) restore() {
	if walk.MsgBox(p.a.mw, t("restore_go_q"), t("restore_confirm"), walk.MsgBoxOKCancel|walk.MsgBoxIconWarning) != walk.DlgCmdOK {
		return
	}
	p.logs = -1
	p.a.showError(p.a.c.StartRestore(p.fullBackup.Checked()))
}

func (p *toolsPage) chooseBackupDir() {
	if dir, ok := chooseFolder(p.a.mw, t("choose_backup"), p.a.c.Settings.Get().Backups()); ok {
		p.a.showError(p.a.c.SetBackupDir(dir))
		p.showBackupDir()
	}
}

func (p *toolsPage) installDriver() {
	if p.driverBusy {
		return
	}
	p.driverBusy = true
	setText(p.driverText, t("driver_installing"))
	dir := p.a.c.StateDir + `\driver`
	go func() {
		err := driver.Install(dir)
		p.a.mw.Synchronize(func() {
			p.driverBusy = false
			p.driverNext = time.Time{}
			if err != nil {
				walk.MsgBox(p.a.form(), t("driver_title"), t("err_driver")+"\r\n\r\n"+err.Error(), walk.MsgBoxIconError)
				return
			}
			walk.MsgBox(p.a.form(), t("driver_title"), t("driver_done"), walk.MsgBoxIconInformation)
		})
	}()
}

// askInstall offers to install the driver as soon as an AI Passport without one
// is plugged in, whichever page is open. It asks once per run; the button on the
// tools page and in the install dialog stays available.
func (p *toolsPage) askInstall() {
	if p.driverBusy || !p.a.driverMissing {
		return
	}
	if walk.MsgBox(p.a.form(), t("driver_title"), t("driver_ask"), walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) == walk.DlgCmdYes {
		p.installDriver()
	}
}

// checkDriver looks for an AI Passport without a driver every few seconds.
func (p *toolsPage) checkDriver() {
	select {
	case status := <-p.driverDone:
		p.a.driverMissing = status == driver.Missing
		if p.a.driverMissing && !p.a.driverAsked && !p.driverBusy {
			p.a.driverAsked = true
			// After this refresh, so the prompt's message loop does not run inside it.
			p.a.mw.Synchronize(p.askInstall)
		}
		if !p.driverBusy {
			if status == driver.Missing {
				setText(p.driverText, t("driver_missing"))
				p.driverText.SetTextColor(red)
			} else {
				setText(p.driverText, t("driver_ok"))
				p.driverText.SetTextColor(grey)
			}
		}
	default:
	}
	if time.Now().Before(p.driverNext) || p.driverBusy {
		return
	}
	p.driverNext = time.Now().Add(3 * time.Second)
	go func() {
		status := driver.Check()
		select {
		case p.driverDone <- status:
		default:
		}
	}()
}

func (p *toolsPage) refresh() {
	p.checkDriver()
	p.refreshInstaller()
	setEnabled(p.driverBtn, !p.driverBusy)
	c := p.a.c
	st := c.Flash.Snapshot()
	busy := st.Active || (c.Sync.Running() && c.Sync.Snapshot().Kind == "move")
	for _, b := range []*walk.PushButton{p.backupBtn, p.restoreBtn, p.backBtn, p.localBtn, p.instBtn} {
		setEnabled(b, !busy)
	}
	setEnabled(p.stopBtn, st.Active && st.Kind != "flash" && !st.Cancelling && cancellable(st.Stage))

	mine := st.Kind == "restore" || st.Kind == "backup"
	if !mine {
		setText(p.stage, t("r_idle"))
		sub := t("r_idle_sub")
		if st.Active && st.Kind == "flash" {
			sub = t("r_busy_flash")
		}
		setText(p.sub, sub)
		setText(p.hint, "")
		setText(p.saved, "")
		setVisible(p.openBtn, false)
		setProgress(p.bar, 0)
		return
	}
	stage, sub, hint := toolStageText(st)
	if p.a.driverMissing && (st.Stage == "waiting" || st.Stage == "failed") {
		hint = t("driver_missing_short")
	}
	setText(p.stage, stage)
	setText(p.sub, sub)
	setText(p.hint, hint)
	setProgress(p.bar, st.Percent)
	saved := ""
	if st.Kind == "backup" && st.File != "" {
		saved = t("b_saved_to") + "：" + st.File
	} else if st.Session != "" && !st.Active {
		saved = t("b_saved_to") + "：" + st.Session
	}
	setText(p.saved, saved)
	setVisible(p.openBtn, st.Session != "" && !st.Active)
	if p.logs != len(st.Logs) {
		p.logs = len(st.Logs)
		p.logView.SetText(crlf(strings.Join(st.Logs, "\n")))
		p.logView.SetTextSelection(len(p.logView.Text()), len(p.logView.Text()))
	}
}

// cancellable lists the stages before anything is written to the device.
func cancellable(stage string) bool {
	switch stage {
	case "preparing", "waiting", "connecting", "checking", "backup":
		return true
	}
	return false
}

// toolStageText is the headline, detail and warning of a restore or backup.
func toolStageText(st flasher.State) (stage, sub, hint string) {
	prefix := "r_"
	if st.Kind == "backup" {
		prefix = "b_"
	}
	key := func(k string) string {
		if i18n.Has(prefix + k) {
			return t(prefix + k)
		}
		if i18n.Has("r_" + k) {
			return t("r_" + k)
		}
		return stageName(k)
	}
	stage = key(st.Stage)
	if i18n.Has(prefix+st.Stage+"_sub") || i18n.Has("r_"+st.Stage+"_sub") {
		sub = key(st.Stage + "_sub")
	}
	switch st.Stage {
	case "waiting":
		if !st.StartedAt.IsZero() {
			sub = i18n.Waited(int(time.Since(st.StartedAt).Seconds()), waitMax)
		}
		switch st.Hint {
		case "multiple":
			hint = t("r_multiple")
		case "code43":
			hint = i18n.Code43(st.Port)
		case "not_seen":
			hint = t("r_not_seen")
		case "replug":
			hint = t("r_replug")
		}
	case "done":
		if st.Kind == "restore" {
			sub = t("r_done_sub")
			if !st.Restarted {
				sub = t("r_done_replug")
			}
			switch st.CardID {
			case "blank_ff", "blank_00":
				hint = t("r_blank_id")
			case "unknown":
				hint = t("r_unknown_id")
			}
		}
		if st.MAC != "" {
			sub += "\r\nMAC: " + st.MAC
		}
	case "failed":
		sub = i18n.ErrText(st.ErrorCode, st.Error)
	}
	return stage, sub, hint
}
