package main

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/jopsam/lara-nux/client/wails/bridge"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	shellEventNavigate    = "shell:navigate"
	shellEventSnapshot    = "shell:snapshot"
	shellEventDaemonError = "shell:daemon-error"
)

type ShellState struct {
	SocketPath   string    `json:"socketPath"`
	Connected    bool      `json:"connected"`
	LastError    string    `json:"lastError,omitempty"`
	LastSyncedAt time.Time `json:"lastSyncedAt,omitempty"`
}

type App struct {
	ctx        context.Context
	client     *bridge.Client
	tray       trayManager
	shellState ShellState
	mu         sync.RWMutex
}

func NewApp(client *bridge.Client, tray trayManager) *App {
	return &App{
		client: client,
		tray:   tray,
		shellState: ShellState{
			SocketPath: client.SocketPath(),
		},
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.tray.Start(trayCallbacks{
		OpenSites: func() { a.focusRoute("/sites") },
		OpenRuntimes: func() { a.focusRoute("/runtimes") },
		Refresh: func() {
			go a.emitSnapshot()
		},
		Quit: func() { a.QuitApplication() },
	})
}

func (a *App) domReady(context.Context) {
	go a.emitSnapshot()
}

func (a *App) shutdown(context.Context) {
	a.tray.Stop()
}

func (a *App) onSecondInstanceLaunch(options.SecondInstanceData) {
	a.focusRoute("/sites")
}

func (a *App) ShowWindow() {
	if a.ctx == nil {
		return
	}
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
	runtime.Show(a.ctx)
}

func (a *App) HideWindow() {
	if a.ctx == nil {
		return
	}
	runtime.WindowHide(a.ctx)
}

func (a *App) QuitApplication() {
	a.tray.Stop()
	if a.ctx == nil {
		os.Exit(0)
	}
	runtime.Quit(a.ctx)
}

func (a *App) GetShellState() ShellState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.shellState
}

func (a *App) LoadDashboard() (bridge.DashboardSnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	health, err := a.client.Health(ctx)
	if err != nil {
		a.markDisconnected(err)
		return bridge.DashboardSnapshot{}, err
	}

	sites, err := a.client.ListSites(ctx)
	if err != nil {
		a.markDisconnected(err)
		return bridge.DashboardSnapshot{}, err
	}

	runtimes, err := a.client.RuntimeCatalog(ctx)
	if err != nil {
		a.markDisconnected(err)
		return bridge.DashboardSnapshot{}, err
	}

	a.markConnected()

	return bridge.DashboardSnapshot{
		Health: health,
		Sites:  sites,
		Runtimes: runtimes,
		Shell: bridge.ShellStatus{
			SocketPath:   a.shellState.SocketPath,
			Connected:    true,
			LastSyncedAt: a.shellState.LastSyncedAt,
		},
	}, nil
}

func (a *App) ListSites() ([]bridge.SiteRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := a.client.ListSites(ctx)
	a.markRequest(err)
	return result, err
}

func (a *App) GetSite(siteID string) (bridge.SiteRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := a.client.GetSite(ctx, siteID)
	a.markRequest(err)
	return result, err
}

func (a *App) RegisterSite(request bridge.RegisterSiteRequest) (bridge.ActivationResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := a.client.RegisterSite(ctx, request)
	a.markRequest(err)
	if err == nil {
		go a.emitSnapshot()
	}
	return result, err
}

func (a *App) UpdateSite(request bridge.UpdateSiteRequest) (bridge.SiteRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := a.client.UpdateSite(ctx, request)
	a.markRequest(err)
	if err == nil {
		go a.emitSnapshot()
	}
	return result, err
}

func (a *App) GetHealth() (bridge.HealthReport, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := a.client.Health(ctx)
	a.markRequest(err)
	return result, err
}

func (a *App) GetRuntimeCatalog() (bridge.RuntimeCatalog, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := a.client.RuntimeCatalog(ctx)
	a.markRequest(err)
	return result, err
}

func (a *App) SetDefaultRuntime(request bridge.SetDefaultPHPRequest) (bridge.DefaultRuntimeResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := a.client.SetDefaultRuntime(ctx, request)
	a.markRequest(err)
	if err == nil {
		go a.emitSnapshot()
	}
	return result, err
}

func (a *App) SwitchSiteRuntime(request bridge.SwitchPHPRequest) (bridge.SiteRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := a.client.SwitchSiteRuntime(ctx, request)
	a.markRequest(err)
	if err == nil {
		go a.emitSnapshot()
	}
	return result, err
}

func (a *App) ServiceAction(request bridge.ServiceActionRequest) (bridge.ServiceStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := a.client.ServiceAction(ctx, request)
	a.markRequest(err)
	if err == nil {
		go a.emitSnapshot()
	}
	return result, err
}

func (a *App) focusRoute(route string) {
	a.ShowWindow()
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, shellEventNavigate, route)
}

func (a *App) emitSnapshot() {
	snapshot, err := a.LoadDashboard()
	if err != nil {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, shellEventDaemonError, err.Error())
		}
		return
	}
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, shellEventSnapshot, snapshot)
	}
}

func (a *App) markRequest(err error) {
	if err != nil {
		a.markDisconnected(err)
		return
	}
	a.markConnected()
}

func (a *App) markConnected() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.shellState.Connected = true
	a.shellState.LastError = ""
	a.shellState.LastSyncedAt = time.Now().UTC()
}

func (a *App) markDisconnected(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.shellState.Connected = false
	a.shellState.LastError = err.Error()
	a.shellState.LastSyncedAt = time.Now().UTC()
}

func defaultSocketPath() string {
	if value := os.Getenv("LARA_NUXT_SOCKET_PATH"); value != "" {
		return value
	}
	return "/run/lara-nux/lara-nux.sock"
}
