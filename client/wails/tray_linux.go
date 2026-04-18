//go:build linux

package main

import (
	"sync"

	"github.com/getlantern/systray"
)

type linuxTrayManager struct {
	startOnce sync.Once
	stopOnce  sync.Once
}

func newTrayManager() trayManager {
	return &linuxTrayManager{}
}

func (m *linuxTrayManager) Start(callbacks trayCallbacks) {
	m.startOnce.Do(func() {
		go systray.Run(func() {
			systray.SetIcon(trayIcon)
			systray.SetTitle("Lara Nux")
			systray.SetTooltip("Lara Nux local Laravel environment")

			openSites := systray.AddMenuItem("Open Sites", "Show the Lara Nux sites workspace")
			openRuntimes := systray.AddMenuItem("Open Runtimes", "Show runtimes and health")
			refresh := systray.AddMenuItem("Refresh Health", "Refresh daemon health and site state")
			systray.AddSeparator()
			quit := systray.AddMenuItem("Quit", "Quit Lara Nux")

			go func() {
				for {
					select {
					case <-openSites.ClickedCh:
						if callbacks.OpenSites != nil {
							callbacks.OpenSites()
						}
					case <-openRuntimes.ClickedCh:
						if callbacks.OpenRuntimes != nil {
							callbacks.OpenRuntimes()
						}
					case <-refresh.ClickedCh:
						if callbacks.Refresh != nil {
							callbacks.Refresh()
						}
					case <-quit.ClickedCh:
						if callbacks.Quit != nil {
							callbacks.Quit()
						}
						return
					}
				}
			}()
		}, func() {})
	})
}

func (m *linuxTrayManager) Stop() {
	m.stopOnce.Do(func() {
		systray.Quit()
	})
}
