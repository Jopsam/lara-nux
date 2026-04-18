//go:build linux && !systray

package main

func newTrayManager() trayManager {
	return noopTrayManager{}
}
