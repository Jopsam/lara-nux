package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jopsam/lara-nux/daemon/internal/api"
	"github.com/jopsam/lara-nux/daemon/internal/app"
	caddyhost "github.com/jopsam/lara-nux/daemon/internal/host/ubuntu/caddy"
	packageshost "github.com/jopsam/lara-nux/daemon/internal/host/ubuntu/packages"
	phphost "github.com/jopsam/lara-nux/daemon/internal/host/ubuntu/php"
	resolvedhost "github.com/jopsam/lara-nux/daemon/internal/host/ubuntu/resolved"
	systemdhost "github.com/jopsam/lara-nux/daemon/internal/host/ubuntu/systemd"
)

const (
	messageBootstrapStarting = "LARANUXT_BOOTSTRAP_STARTING"
	messagePreflightPassed   = "LARANUXT_BOOTSTRAP_PREFLIGHT_PASSED"
	messageSocketReady       = "LARANUXT_SOCKET_READY"
	messageShutdown          = "LARANUXT_DAEMON_SHUTDOWN"
	messageBootstrapFailed   = "LARANUXT_BOOTSTRAP_FAILED"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	paths := app.LoadPathsFromEnv()
	logger := journaldLogger{identifier: "lara-nuxd"}

	logger.Event(messageBootstrapStarting, "Starting privileged bootstrap.", map[string]string{
		"config_dir":          paths.ConfigDir,
		"state_dir":           paths.StateDir,
		"runtime_dir":         paths.RuntimeDir,
		"socket_path":         paths.SocketPath,
		"managed_assets_path": paths.ManagedAssetsPath,
	})

	bootstrap := app.NewBootstrapService(paths)
	report, err := bootstrap.Preflight(ctx)
	if err != nil {
		logger.Event(messageBootstrapFailed, "Bootstrap preflight failed.", map[string]string{
			"error":        err.Error(),
			"host_id":      report.Host.ID,
			"host_version": report.Host.VersionID,
		})
		os.Exit(1)
	}

	if err := bootstrap.PrepareFilesystem(); err != nil {
		logger.Event(messageBootstrapFailed, "Bootstrap filesystem preparation failed.", map[string]string{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	if err := report.Manifest.Save(paths.ManagedAssetsPath); err != nil {
		logger.Event(messageBootstrapFailed, "Managed asset manifest persistence failed.", map[string]string{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	logger.Event(messagePreflightPassed, "Bootstrap preflight passed.", map[string]string{
		"supported_release": report.Host.VersionID,
		"managed_assets":    strconv.Itoa(len(report.Manifest.Assets)),
	})

	listener, err := bindUnixSocket(paths.SocketPath)
	if err != nil {
		logger.Event(messageBootstrapFailed, "Unix socket binding failed.", map[string]string{
			"socket_path": paths.SocketPath,
			"error":       err.Error(),
		})
		os.Exit(1)
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(paths.SocketPath)
	}()

	logger.Event(messageSocketReady, "Daemon socket bound and ready for RPC traffic.", map[string]string{
		"socket_path": paths.SocketPath,
		"ready":       "1",
	})

	siteRegistry := app.NewSiteRegistryFromPaths(paths)
	phpRegistry := app.NewPHPRegistryFromPaths(paths)
	hostServiceManager := systemdhost.NewManager(systemdhost.Config{})
	serviceManager := app.NewServiceManager(hostServiceManager)
	resolverManager := resolvedhost.NewManager(resolvedhost.Config{})
	webManager := caddyhost.NewManager(caddyhost.Config{})
	hostPHPManager := phphost.NewManager(phphost.Config{})
	packageManager := packageshost.NewManager(packageshost.Config{})
	phpManager := app.NewPHPManager(siteRegistry, phpRegistry, serviceManager, hostPHPManager, webManager)
	siteActivation := app.NewSiteActivationService(siteRegistry, phpRegistry, resolverManager, webManager, hostPHPManager, serviceManager)
	siteManagement := app.NewSiteManagementService(siteRegistry, phpRegistry, webManager, hostPHPManager, serviceManager)
	runtimeOnboarding := app.NewRuntimeOnboardingService(phpRegistry, packageManager, phpRegistry, hostPHPManager, serviceManager)
	runtimeCatalog := app.NewRuntimeCatalogService(phpRegistry, packageManager)
	healthService := app.NewHealthService(siteRegistry, phpRegistry, serviceManager, resolverManager, paths.SocketPath)

	router := api.NewRouter(api.RouterDependencies{
		HealthService:         healthService,
		PHPManager:            phpManager,
		ServiceManager:        serviceManager,
		SiteActivationService: siteActivation,
		SiteManagementService: siteManagement,
		RuntimeOnboarding:     runtimeOnboarding,
		RuntimeCatalogService: runtimeCatalog,
		ResolverManager:       resolverManager,
	})

	server := &http.Server{Handler: router}
	serverErr := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		logger.Event(messageShutdown, "Shutdown signal received.", map[string]string{
			"signal": ctx.Err().Error(),
		})
	case err := <-serverErr:
		logger.Event(messageBootstrapFailed, "RPC server failed while serving the daemon socket.", map[string]string{
			"error": err.Error(),
		})
		os.Exit(1)
	}
}

func bindUnixSocket(socketPath string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o750); err != nil {
		return nil, fmt.Errorf("create runtime directory: %w", err)
	}

	if info, err := os.Lstat(socketPath); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("existing path is not a unix socket: %s", socketPath)
		}

		if err := os.Remove(socketPath); err != nil {
			return nil, fmt.Errorf("remove stale unix socket: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("inspect socket path: %w", err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("listen on unix socket: %w", err)
	}

	if err := os.Chmod(socketPath, 0o660); err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("set socket permissions: %w", err)
	}

	group, err := user.LookupGroup("lara-nux")
	if err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("lookup socket group: %w", err)
	}

	gid, err := strconv.Atoi(group.Gid)
	if err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("parse socket group id: %w", err)
	}

	if err := os.Chown(socketPath, os.Geteuid(), gid); err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("set socket ownership: %w", err)
	}

	return listener, nil
}

type journaldLogger struct {
	identifier string
}

func (l journaldLogger) Event(messageID string, message string, fields map[string]string) {
	parts := []string{
		fmt.Sprintf("SYSLOG_IDENTIFIER=%s", strconv.Quote(l.identifier)),
		fmt.Sprintf("MESSAGE_ID=%s", strconv.Quote(messageID)),
		fmt.Sprintf("MESSAGE=%s", strconv.Quote(message)),
	}

	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", sanitizeJournalKey(key), strconv.Quote(fields[key])))
	}

	log.Print(strings.Join(parts, " "))
}

func sanitizeJournalKey(value string) string {
	value = strings.ToUpper(value)
	replacer := strings.NewReplacer("-", "_", ".", "_", "/", "_", " ", "_")
	return replacer.Replace(value)
}
