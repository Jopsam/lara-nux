package main

import (
	stdRuntime "runtime"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
)

func (a *App) menu() *menu.Menu {
	appMenu := menu.NewMenu()
	if stdRuntime.GOOS == "darwin" {
		appMenu.Append(menu.AppMenu())
	}

	windowMenu := appMenu.AddSubmenu("Lara Nux")
	windowMenu.AddText("Show window", keys.CmdOrCtrl("shift+s"), func(_ *menu.CallbackData) {
		a.focusRoute("/sites")
	})
	windowMenu.AddText("Refresh health", keys.CmdOrCtrl("r"), func(_ *menu.CallbackData) {
		go a.emitSnapshot()
	})
	windowMenu.AddSeparator()
	windowMenu.AddText("Quit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		a.QuitApplication()
	})

	if stdRuntime.GOOS == "darwin" {
		appMenu.Append(menu.EditMenu())
	}

	return appMenu
}
