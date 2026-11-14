//go:build windows

// AI Passport 玩法社区 for Windows 7 / 8 / 8.1: a native Win32 client. Flashing,
// factory restore, backups and the play library come from the shared packages of
// github.com/FoloToy/ai-passport-desktop (pkg/core and below); this program adds the
// window (internal/ui) and the USB serial driver installer (internal/driver).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/FoloToy/ai-passport-desktop/pkg/core"
	"github.com/FoloToy/ai-passport-desktop/pkg/device"
	"github.com/FoloToy/ai-passport-desktop/pkg/esp"
	"github.com/FoloToy/ai-passport-desktop/pkg/factory"
	"github.com/FoloToy/ai-passport-desktop/pkg/flasher"
	"github.com/FoloToy/ai-passport-desktop/pkg/instance"
	"github.com/FoloToy/ai-passport-desktop/pkg/settings"
	"github.com/FoloToy/ai-passport-desktop/pkg/syncer"

	"github.com/FoloToy/ai-passport-desktop-windows7/internal/driver"
	"github.com/FoloToy/ai-passport-desktop-windows7/internal/i18n"
	"github.com/FoloToy/ai-passport-desktop-windows7/internal/ui"
)

var version = "0.1.0-dev"

func main() {
	selfTest := flag.Bool("self-test", false, "Check the bundled flasher stub, factory firmware and driver, print JSON and exit")
	stateDir := flag.String("state-dir", "", "Override the private settings directory")
	origin := flag.String("origin", syncer.DefaultOrigin, "Community site origin")
	flag.Parse()
	if *selfTest {
		runSelfTest()
		return
	}

	dir := *stateDir
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			ui.Notice(err.Error())
			os.Exit(1)
		}
		// Same folder as the desktop app and the old Chrome-based Windows 7
		// edition, so an existing library and settings are kept.
		dir = filepath.Join(base, "FoloToy", "AI Passport Plays")
	}
	store, err := settings.Open(dir)
	if err != nil {
		ui.Notice(err.Error())
		os.Exit(1)
	}
	i18n.Set(i18n.Pick(store.Get().Language, settings.SystemLanguage()))

	unlock, err := instance.Acquire(dir)
	if err != nil {
		if !ui.Activate() {
			ui.Notice(i18n.T("already_running"))
		}
		return
	}
	defer unlock()

	runner := device.Runner{}
	sync := syncer.New(syncer.NewClient(*origin, version))
	c := core.New(version, dir, store, sync, flasher.New(runner), runner)
	c.CheckUpdatesInBackground()
	if err := ui.Run(c); err != nil {
		ui.Notice(err.Error())
		os.Exit(1)
	}
}

// runSelfTest is used by the packaging script to check the built program.
func runSelfTest() {
	stub, err := esp.StubSize()
	if err == nil {
		err = factory.Verify()
	}
	if err == nil && len(driver.INF) == 0 {
		err = fmt.Errorf("driver INF is empty")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "self-test failed:", err)
		os.Exit(1)
	}
	fmt.Printf(`{"passed":true,"version":%q,"edition":"win7-native","chip":"ESP32-C3","stub_bytes":%d,"factory":%q}`+"\n", version, stub, factory.Version)
}
