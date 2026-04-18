package main

type trayCallbacks struct {
	OpenSites    func()
	OpenRuntimes func()
	Refresh      func()
	Quit         func()
}

type trayManager interface {
	Start(callbacks trayCallbacks)
	Stop()
}

type noopTrayManager struct{}

func (noopTrayManager) Start(trayCallbacks) {}
func (noopTrayManager) Stop()               {}
