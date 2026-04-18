//go:build !linux

package main

func newTrayManager() trayManager {
	return noopTrayManager{}
}
