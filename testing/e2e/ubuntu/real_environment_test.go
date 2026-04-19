package ubuntu

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jopsam/lara-nux/daemon/internal/app"
	packageshost "github.com/jopsam/lara-nux/daemon/internal/host/ubuntu/packages"
)

const realUbuntuE2EEnv = "LARA_NUX_REAL_UBUNTU_E2E"

func TestUbuntuLTSRealEnvironmentWorkflow(t *testing.T) {
	if os.Getenv(realUbuntuE2EEnv) == "" {
		t.Skip("set LARA_NUX_REAL_UBUNTU_E2E=1 to run real Ubuntu container E2E")
	}

	for _, fixtureName := range []string{"jammy.json", "noble.json"} {
		fixtureName := fixtureName
		t.Run(strings.TrimSuffix(fixtureName, filepath.Ext(fixtureName)), func(t *testing.T) {
			scenario := loadScenario(t, fixtureName)
			runUbuntuRealEnvironmentWorkflow(t, scenario)
		})
	}
}

func runUbuntuRealEnvironmentWorkflow(t *testing.T, scenario workflowScenario) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Minute)
	defer cancel()

	env := newUbuntuHarness(t, ctx, repoRoot(t), scenario)
	defer env.Close()

	env.installHostPackages()
	env.installDebPackage()
	env.waitForDaemonSocket()

	projectRoot := env.createLaravelProject()
	env.registerRuntime(scenario.DefaultPHP)
	env.registerRuntime(scenario.SwitchPHP)
	env.setDefaultRuntime(scenario.DefaultPHP)

	activation := env.registerSite(projectRoot)
	if activation.Site.Domain != scenario.Domain {
		t.Fatalf("expected domain %s, got %s", scenario.Domain, activation.Site.Domain)
	}
	if activation.Web.HTTPSURL != "https://"+scenario.Domain {
		t.Fatalf("expected HTTPS URL https://%s, got %s", scenario.Domain, activation.Web.HTTPSURL)
	}

	env.waitForHTTPSReady()
	env.assertHTTPSReturnsPHPVersion(scenario.DefaultPHP)
	env.assertResolverStubInstalled()
	env.assertManagedArtifactsPresent(scenario.DefaultPHP)

	switched := env.switchRuntime(activation.Site.ID, scenario.SwitchPHP)
	if switched.PHPVersion != scenario.SwitchPHP {
		t.Fatalf("expected switched PHP %s, got %s", scenario.SwitchPHP, switched.PHPVersion)
	}

	env.waitForHTTPSReady()
	env.assertHTTPSReturnsPHPVersion(scenario.SwitchPHP)
	env.assertManagedArtifactsPresent(scenario.SwitchPHP)

	env.uninstallPackage()
	env.assertManagedArtifactsRemoved()
	env.assertProjectPreserved(projectRoot)
}

type ubuntuHarness struct {
	t             *testing.T
	ctx           context.Context
	repoRoot      string
	scenario      workflowScenario
	imageTag      string
	containerName string
	sessionDir    string
	debPath       string
	httpClient    *http.Client
	containerIP   string
}

func newUbuntuHarness(t *testing.T, ctx context.Context, repoRoot string, scenario workflowScenario) *ubuntuHarness {
	t.Helper()

	containerName := fmt.Sprintf("lara-nux-%s-%d", scenario.Codename, time.Now().UTC().UnixNano())
	sessionDir := filepath.Join(repoRoot, ".tmp", "real-ubuntu-e2e", containerName)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir harness dir: %v", err)
	}

	imageTag := "lara-nux-systemd-" + scenario.Codename
	buildUbuntuSystemdImage(t, ctx, repoRoot, scenario, imageTag)
	debPath := buildValidationDeb(t, repoRoot)

	runCommand(t, ctx, repoRoot, "docker", "run", "-d",
		"--privileged",
		"--cgroupns=host",
		"--tmpfs", "/run",
		"--tmpfs", "/run/lock",
		"--tmpfs", "/tmp",
		"-v", "/sys/fs/cgroup:/sys/fs/cgroup:rw",
		"-v", repoRoot+":/workspace",
		"-v", sessionDir+":/session",
		"--name", containerName,
		imageTag,
	)

	ip := inspectContainerIP(t, ctx, repoRoot, containerName)
	client := &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(ip, "443"))
			},
		},
	}

	return &ubuntuHarness{
		t:             t,
		ctx:           ctx,
		repoRoot:      repoRoot,
		scenario:      scenario,
		imageTag:      imageTag,
		containerName: containerName,
		sessionDir:    sessionDir,
		debPath:       debPath,
		httpClient:    client,
		containerIP:   ip,
	}
}

func (e *ubuntuHarness) Close() {
	if e.t.Failed() {
		e.t.Logf("retaining failed container %s for inspection", e.containerName)
		e.t.Logf("caddy status:\n%s", tryRunHost(e.ctx, e.repoRoot, "docker", "exec", e.containerName, "bash", "-lc", "systemctl status caddy --no-pager || true"))
		e.t.Logf("caddy journal:\n%s", tryRunHost(e.ctx, e.repoRoot, "docker", "exec", e.containerName, "bash", "-lc", "journalctl -u caddy --no-pager -n 200 || true"))
		e.t.Logf("root Caddyfile:\n%s", tryRunHost(e.ctx, e.repoRoot, "docker", "exec", e.containerName, "bash", "-lc", "cat /etc/caddy/Caddyfile || true"))
		e.t.Logf("managed site configs:\n%s", tryRunHost(e.ctx, e.repoRoot, "docker", "exec", e.containerName, "bash", "-lc", "for f in /etc/caddy/sites.d/lara-nux/*.caddy; do printf '--- %s ---\n' \"$f\"; cat \"$f\"; printf '\n'; done 2>/dev/null || true"))
		if os.Getenv("LARA_NUX_KEEP_FAILED_UBUNTU_E2E") != "1" {
			_ = tryRunHost(e.ctx, e.repoRoot, "docker", "rm", "-f", e.containerName)
		}
		return
	}
	_ = tryRunHost(e.ctx, e.repoRoot, "docker", "rm", "-f", e.containerName)
}

func (e *ubuntuHarness) installHostPackages() {
	e.exec("DEBIAN_FRONTEND=noninteractive apt-get -o Dpkg::Use-Pty=0 update")
	e.exec("DEBIAN_FRONTEND=noninteractive apt-get -o Dpkg::Use-Pty=0 install -y --no-install-recommends dbus debian-keyring debian-archive-keyring python3-launchpadlib")
	e.exec("curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg")
	e.exec("curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' > /etc/apt/sources.list.d/caddy-stable.list")
	e.exec("add-apt-repository -y ppa:ondrej/php")
	e.exec("DEBIAN_FRONTEND=noninteractive apt-get -o Dpkg::Use-Pty=0 update")

	manager := packageshost.NewManager(packageshost.Config{})
	packageMap := map[string][]string{}
	for _, supported := range manager.SupportedPackages() {
		packageMap[supported.Key] = supported.Packages
	}

	wantedKeys := []string{"caddy", "php-" + e.scenario.DefaultPHP}
	if e.scenario.SwitchPHP != e.scenario.DefaultPHP {
		wantedKeys = append(wantedKeys, "php-"+e.scenario.SwitchPHP)
	}

	for _, key := range wantedKeys {
		packages, ok := packageMap[key]
		if !ok {
			e.t.Fatalf("missing supported package definition for %s", key)
		}
		e.exec("DEBIAN_FRONTEND=noninteractive apt-get -o Dpkg::Use-Pty=0 install -y --no-install-recommends " + strings.Join(packages, " "))
	}

	e.exec("if ! command -v resolvectl >/dev/null 2>&1; then if apt-cache show systemd-resolved >/dev/null 2>&1; then DEBIAN_FRONTEND=noninteractive apt-get -o Dpkg::Use-Pty=0 install -y --no-install-recommends systemd-resolved; fi; fi")

	e.exec("systemctl daemon-reload")
	e.exec("if systemctl list-unit-files systemd-resolved.service --no-legend 2>/dev/null | grep -q 'systemd-resolved.service'; then systemctl reset-failed systemd-resolved || true; systemctl start systemd-resolved; fi")
	e.exec("systemctl reset-failed caddy || true")
	e.exec("systemctl start caddy")
	e.waitUntil(30*time.Second, func() bool {
		return strings.Contains(e.execOutput("systemctl is-active caddy || true"), "active")
	}, "caddy to become active")

	e.waitUntil(30*time.Second, func() bool {
		return strings.Contains(e.execOutput("systemctl is-active systemd-resolved || true"), "active")
	}, "systemd-resolved to become active")

	if !strings.Contains(e.execOutput("command -v resolvectl"), "/usr/bin/resolvectl") {
		e.t.Fatal("expected resolvectl to be available")
	}
	if !strings.Contains(e.execOutput("grep '^User=' /lib/systemd/system/caddy.service"), "User=caddy") {
		e.t.Fatal("expected packaged caddy unit to run as caddy user")
	}
}

func (e *ubuntuHarness) installDebPackage() {
	containerDeb := "/session/" + filepath.Base(e.debPath)
	copyFile(e.t, e.debPath, filepath.Join(e.sessionDir, filepath.Base(e.debPath)))
	e.exec("dpkg -i " + containerDeb + " || true")
	e.waitUntil(45*time.Second, func() bool {
		return strings.Contains(e.execOutput("systemctl is-active lara-nuxd.service || true"), "active")
	}, "lara-nuxd service to become active")

	if !strings.Contains(e.execOutput("getent group lara-nux"), "lara-nux") {
		e.t.Fatal("expected lara-nux group to exist after package install")
	}
	if !strings.Contains(e.execOutput("getent passwd lara-nuxd"), "lara-nuxd") {
		e.t.Fatal("expected lara-nuxd user to exist after package install")
	}
	if !strings.Contains(e.execOutput("stat -c '%U:%G %a' /run/lara-nux"), "root:lara-nux 770") {
		e.t.Fatalf("unexpected /run/lara-nux ownership: %s", e.execOutput("stat -c '%U:%G %a' /run/lara-nux"))
	}
	if !strings.Contains(e.execOutput("grep -E '^ExecStart=/usr/bin/lara-nuxd' /lib/systemd/system/lara-nuxd.service"), "/usr/bin/lara-nuxd") {
		e.t.Fatal("expected installed lara-nuxd systemd unit")
	}
	if !strings.Contains(e.execOutput("grep -R 'SignWith: CHANGE_ME' /usr/share/lara-nux/packaging/repo/distributions"), "SignWith: CHANGE_ME") {
		e.t.Fatal("expected packaged repo metadata placeholder to be present")
	}
	if !strings.Contains(e.execOutput("grep -R 'remove_glob_if_managed' /var/lib/dpkg/info/lara-nux.postrm"), "remove_glob_if_managed") {
		e.t.Fatal("expected packaged postrm script to contain managed cleanup hooks")
	}
}

func (e *ubuntuHarness) waitForDaemonSocket() {
	e.waitUntil(45*time.Second, func() bool {
		return strings.Contains(e.execOutput("test -S /run/lara-nux/lara-nux.sock && echo ready || true"), "ready")
	}, "lara-nux unix socket")

	if !strings.Contains(e.execOutput("stat -c '%U:%G %a' /run/lara-nux/lara-nux.sock"), "root:lara-nux 660") {
		e.t.Fatalf("unexpected socket permissions: %s", e.execOutput("stat -c '%U:%G %a' /run/lara-nux/lara-nux.sock"))
	}
	if !strings.Contains(e.execOutput("curl --silent --unix-socket /run/lara-nux/lara-nux.sock http://localhost/rpc/health"), "\"ok\":true") {
		e.t.Fatal("expected daemon health RPC over unix socket")
	}
}

func (e *ubuntuHarness) createLaravelProject() string {
	root := "/workspace/.tmp/real-laravel-" + e.scenario.Codename
	e.exec(fmt.Sprintf(`rm -rf %s && mkdir -p %s && cat > %s <<'EOF'
#!/usr/bin/env php
<?php echo 'artisan';
EOF
chmod 0755 %s && cat > %s <<'EOF'
{}
EOF
cat > %s <<'EOF'
<?php
header('Content-Type: text/plain');
echo PHP_MAJOR_VERSION . '.' . PHP_MINOR_VERSION;
EOF`,
		shellQuote(root),
		shellQuote(filepath.Join(root, "public")),
		shellQuote(filepath.Join(root, "artisan")),
		shellQuote(filepath.Join(root, "artisan")),
		shellQuote(filepath.Join(root, "composer.json")),
		shellQuote(filepath.Join(root, "public", "index.php")),
	))
	return root
}

func (e *ubuntuHarness) registerRuntime(version string) {
	version = app.NormalizePHPVersion(version)
	response := e.rpc(http.MethodPost, "/rpc/php.register", map[string]any{
		"version":    version,
		"binaryPath": "/usr/bin/php" + version,
		"fpmService": "php" + version + "-fpm",
		"source":     e.scenario.Codename,
	})
	if response.StatusCode != http.StatusCreated {
		e.t.Fatalf("register runtime %s failed: %s", version, response.Body)
	}
	if !strings.Contains(e.execOutput("systemctl is-active php"+version+"-fpm || true"), "active") {
		e.t.Fatalf("expected php%s-fpm active after runtime registration", version)
	}
	if !strings.Contains(e.execOutput("test -f /etc/php/"+version+"/fpm/pool.d/lara-nux.conf && echo ok || true"), "ok") {
		e.t.Fatalf("expected managed php pool for %s", version)
	}
	if !strings.Contains(e.execOutput("grep -R 'listen.owner = caddy' /etc/php/"+version+"/fpm/pool.d/lara-nux.conf"), "listen.owner = caddy") {
		e.t.Fatalf("expected php pool ownership for caddy on %s", version)
	}
}

func (e *ubuntuHarness) setDefaultRuntime(version string) {
	response := e.rpc(http.MethodPost, "/rpc/php.default", map[string]any{"version": version})
	if response.StatusCode != http.StatusOK {
		e.t.Fatalf("set default runtime %s failed: %s", version, response.Body)
	}
}

func (e *ubuntuHarness) registerSite(projectRoot string) app.ActivationResult {
	response := e.rpc(http.MethodPost, "/rpc/sites.register", map[string]any{
		"rootPath":   filepath.Join(projectRoot, "."),
		"domain":     e.scenario.Domain,
		"phpVersion": e.scenario.DefaultPHP,
	})
	if response.StatusCode != http.StatusCreated {
		e.t.Fatalf("register site failed: %s", response.Body)
	}
	var envelope struct {
		OK   bool                 `json:"ok"`
		Data app.ActivationResult `json:"data"`
	}
	decodeJSONBody(e.t, response.BodyBytes(), &envelope)
	return envelope.Data
}

func (e *ubuntuHarness) switchRuntime(siteID string, version string) app.SiteRecord {
	response := e.rpc(http.MethodPost, "/rpc/php.switch", map[string]any{"siteId": siteID, "phpVersion": version})
	if response.StatusCode != http.StatusOK {
		e.t.Fatalf("switch runtime failed: %s", response.Body)
	}
	var envelope struct {
		OK   bool          `json:"ok"`
		Data app.SiteRecord `json:"data"`
	}
	decodeJSONBody(e.t, response.BodyBytes(), &envelope)
	return envelope.Data
}

func (e *ubuntuHarness) waitForHTTPSReady() {
	e.waitUntil(90*time.Second, func() bool {
		request, err := http.NewRequestWithContext(e.ctx, http.MethodGet, "https://"+e.scenario.Domain, nil)
		if err != nil {
			return false
		}
		request.Host = e.scenario.Domain
		response, err := e.httpClient.Do(request)
		if err != nil {
			return false
		}
		defer response.Body.Close()
		return response.StatusCode == http.StatusOK
	}, "HTTPS response")
}

func (e *ubuntuHarness) assertHTTPSReturnsPHPVersion(version string) {
	request, err := http.NewRequestWithContext(e.ctx, http.MethodGet, "https://"+e.scenario.Domain, nil)
	if err != nil {
		e.t.Fatalf("build HTTPS request: %v", err)
	}
	request.Host = e.scenario.Domain
	response, err := e.httpClient.Do(request)
	if err != nil {
		e.t.Fatalf("perform HTTPS request: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		e.t.Fatalf("read HTTPS response: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		e.t.Fatalf("expected HTTPS 200, got %d: %s", response.StatusCode, string(body))
	}
	if response.TLS == nil {
		e.t.Fatal("expected HTTPS TLS state")
	}
	if !strings.Contains(string(body), app.NormalizePHPVersion(version)) {
		e.t.Fatalf("expected HTTPS body to contain PHP %s, got %q", version, string(body))
	}
}

func (e *ubuntuHarness) assertResolverStubInstalled() {
	if !strings.Contains(e.execOutput("grep -R 'Domains=~test' /etc/systemd/resolved.conf.d/lara-nux-test.conf"), "Domains=~test") {
		e.t.Fatal("expected resolver stub to declare Domains=~test")
	}
	if !strings.Contains(e.execOutput("systemctl is-active systemd-resolved || true"), "active") {
		e.t.Fatal("expected systemd-resolved to be active")
	}
}

func (e *ubuntuHarness) assertManagedArtifactsPresent(version string) {
	version = app.NormalizePHPVersion(version)
	checks := map[string]string{
		"daemon socket":      "test -S /run/lara-nux/lara-nux.sock && echo ok || true",
		"resolver stub":      "test -f /etc/systemd/resolved.conf.d/lara-nux-test.conf && echo ok || true",
		"managed manifest":   "test -f /var/lib/lara-nux/managed-assets.json && echo ok || true",
		"caddy root import":  "grep -R 'Lara Nux managed Caddy import start' /etc/caddy/Caddyfile && echo ok || true",
		"caddy site file":    "ls /etc/caddy/sites.d/lara-nux/*.caddy >/dev/null 2>&1 && echo ok || true",
		"php pool file":      "test -f /etc/php/" + version + "/fpm/pool.d/lara-nux.conf && echo ok || true",
		"php override file":  "test -f /etc/systemd/system/php" + version + "-fpm.d/lara-nux.conf && echo ok || test -f /etc/systemd/system/php" + version + "-fpm.service.d/lara-nux.conf && echo ok || true",
	}
	for name, command := range checks {
		if !strings.Contains(e.execOutput(command), "ok") {
			e.t.Fatalf("expected %s to be present", name)
		}
	}
	if !strings.Contains(e.execOutput("grep -R '/run/php/lara-nux-php"+version+"-fpm.sock' /etc/caddy/sites.d/lara-nux/*.caddy"), "/run/php/lara-nux-php"+version+"-fpm.sock") {
		e.t.Fatalf("expected caddy config to point at PHP %s socket", version)
	}
	if !strings.Contains(e.execOutput("systemctl is-active caddy || true"), "active") {
		e.t.Fatal("expected caddy to be active")
	}
	if !strings.Contains(e.execOutput("systemctl is-active php"+version+"-fpm || true"), "active") {
		e.t.Fatalf("expected php%s-fpm to be active", version)
	}
}

func (e *ubuntuHarness) uninstallPackage() {
	e.exec("DEBIAN_FRONTEND=noninteractive apt-get remove -y lara-nux")
	e.exec("DEBIAN_FRONTEND=noninteractive apt-get purge -y lara-nux")
	if strings.Contains(e.execOutput("dpkg -l | grep '^ii  lara-nux' || true"), "lara-nux") {
		e.t.Fatal("expected lara-nux package to be removed")
	}
}

func (e *ubuntuHarness) assertManagedArtifactsRemoved() {
	paths := []string{
		"/run/lara-nux/lara-nux.sock",
		"/etc/systemd/resolved.conf.d/lara-nux-test.conf",
		"/var/lib/lara-nux/managed-assets.json",
	}
	for _, path := range paths {
		if strings.Contains(e.execOutput("test -e "+path+" && echo exists || true"), "exists") {
			e.t.Fatalf("expected managed path %s to be removed", path)
		}
	}
	if strings.Contains(e.execOutput("ls /etc/caddy/sites.d/lara-nux/*.caddy 2>/dev/null || true"), ".caddy") {
		e.t.Fatal("expected managed caddy site configs to be removed")
	}
	if strings.Contains(e.execOutput("getent passwd lara-nuxd || true"), "lara-nuxd") {
		e.t.Fatal("expected lara-nuxd user to be removed on purge")
	}
	if strings.Contains(e.execOutput("getent group lara-nux || true"), "lara-nux") {
		e.t.Fatal("expected lara-nux group to be removed on purge")
	}
}

func (e *ubuntuHarness) assertProjectPreserved(projectRoot string) {
	if !strings.Contains(e.execOutput("test -f "+shellQuote(filepath.Join(projectRoot, "artisan"))+" && echo ok || true"), "ok") {
		e.t.Fatalf("expected project %s to survive uninstall", projectRoot)
	}
}

func (e *ubuntuHarness) rpc(method string, path string, payload any) rpcResponse {
	command := "curl --silent --show-error --write-out '\n%{http_code}' --unix-socket /run/lara-nux/lara-nux.sock"
	if method != http.MethodGet {
		command += " -X " + shellQuote(method)
	}
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			e.t.Fatalf("marshal rpc payload: %v", err)
		}
		command += " -H 'Content-Type: application/json' --data-raw " + shellQuote(string(encoded))
	}
	command += " http://localhost" + path
	output := e.execOutput(command)
	parts := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(parts) == 0 {
		e.t.Fatalf("empty rpc response for %s %s", method, path)
	}
	statusLine := parts[len(parts)-1]
	body := strings.Join(parts[:len(parts)-1], "\n")
	statusCode := 0
	if _, err := fmt.Sscanf(statusLine, "%d", &statusCode); err != nil {
		e.t.Fatalf("parse rpc status %q: %v\nfull output: %s", statusLine, err, output)
	}
	return rpcResponse{StatusCode: statusCode, Body: body, body: []byte(body)}
}

func (e *ubuntuHarness) exec(command string) {
	e.t.Helper()
	runCommand(e.t, e.ctx, e.repoRoot, "docker", "exec", e.containerName, "bash", "-lc", command)
}

func (e *ubuntuHarness) execOutput(command string) string {
	e.t.Helper()
	return runCommand(e.t, e.ctx, e.repoRoot, "docker", "exec", e.containerName, "bash", "-lc", command)
}

func (e *ubuntuHarness) waitUntil(timeout time.Duration, check func() bool, description string) {
	e.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(1 * time.Second)
	}
	e.t.Fatalf("timed out waiting for %s", description)
}

type rpcResponse struct {
	StatusCode int
	Body       string
	body       []byte
}

func (r rpcResponse) BodyBytes() []byte { return r.body }

func buildUbuntuSystemdImage(t *testing.T, ctx context.Context, repoRoot string, scenario workflowScenario, tag string) {
	t.Helper()
	runCommand(t, ctx, repoRoot,
		"docker", "build",
		"-t", tag,
		"-f", filepath.Join("testing", "e2e", "ubuntu", "harness", "ubuntu-systemd.Dockerfile"),
		"--build-arg", "UBUNTU_VERSION="+strings.TrimPrefix(scenario.Name, "ubuntu-"),
		".",
	)
}

func buildValidationDeb(t *testing.T, repoRoot string) string {
	t.Helper()
	workspace := filepath.Join(repoRoot, ".tmp", "real-ubuntu-e2e", "deb-build")
	if err := os.RemoveAll(workspace); err != nil {
		t.Fatalf("reset deb workspace: %v", err)
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir deb workspace: %v", err)
	}
	daemonBinary := filepath.Join(workspace, "lara-nuxd")
	runCommandEnv(t, context.Background(), filepath.Join(repoRoot, "daemon"), map[string]string{
		"CGO_ENABLED": "0",
		"GOOS":        "linux",
		"GOARCH":      "amd64",
	}, "go", "build", "-o", daemonBinary, "./cmd/lara-nuxd")

	packageRoot := filepath.Join(workspace, "pkgroot")
	debianDir := filepath.Join(packageRoot, "DEBIAN")
	paths := []string{
		filepath.Join(packageRoot, "usr", "bin"),
		filepath.Join(packageRoot, "usr", "lib", "lara-nux", "client"),
		filepath.Join(packageRoot, "lib", "systemd", "system"),
		filepath.Join(packageRoot, "usr", "share", "lara-nux", "packaging", "repo"),
		filepath.Join(packageRoot, "usr", "share", "lara-nux", "packaging"),
		debianDir,
	}
	for _, path := range paths {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir package path %s: %v", path, err)
		}
	}

	copyFile(t, daemonBinary, filepath.Join(packageRoot, "usr", "bin", "lara-nuxd"))
	if err := os.Chmod(filepath.Join(packageRoot, "usr", "bin", "lara-nuxd"), 0o755); err != nil {
		t.Fatalf("chmod packaged daemon: %v", err)
	}
	if err := os.WriteFile(filepath.Join(packageRoot, "usr", "lib", "lara-nux", "client", "index.html"), []byte("<!doctype html><html><body>Lara Nux E2E harness</body></html>\n"), 0o644); err != nil {
		t.Fatalf("write packaged client placeholder: %v", err)
	}

	for _, pair := range [][2]string{
		{filepath.Join(repoRoot, "packaging", "ubuntu", "systemd", "lara-nuxd.service"), filepath.Join(packageRoot, "lib", "systemd", "system", "lara-nuxd.service")},
		{filepath.Join(repoRoot, "packaging", "ubuntu", "debian", "lara-nux.managed-files"), filepath.Join(packageRoot, "usr", "share", "lara-nux", "packaging", "lara-nux.managed-files")},
		{filepath.Join(repoRoot, "packaging", "ubuntu", "repo", "apt-ftparchive.conf"), filepath.Join(packageRoot, "usr", "share", "lara-nux", "packaging", "repo", "apt-ftparchive.conf")},
		{filepath.Join(repoRoot, "packaging", "ubuntu", "repo", "distributions"), filepath.Join(packageRoot, "usr", "share", "lara-nux", "packaging", "repo", "distributions")},
		{filepath.Join(repoRoot, "packaging", "ubuntu", "scripts", "postinst"), filepath.Join(debianDir, "postinst")},
		{filepath.Join(repoRoot, "packaging", "ubuntu", "scripts", "prerm"), filepath.Join(debianDir, "prerm")},
		{filepath.Join(repoRoot, "packaging", "ubuntu", "scripts", "postrm"), filepath.Join(debianDir, "postrm")},
	} {
		copyFile(t, pair[0], pair[1])
	}
	for _, script := range []string{"postinst", "prerm", "postrm"} {
		if err := os.Chmod(filepath.Join(debianDir, script), 0o755); err != nil {
			t.Fatalf("chmod %s: %v", script, err)
		}
	}

	control := strings.TrimSpace(`Package: lara-nux
Version: 0.1.0~beta1-1
Section: devel
Priority: optional
Architecture: amd64
Maintainer: Lara Nux Maintainers <opensource@users.noreply.github.com>
Description: Ubuntu-first local Laravel environment for developers
 Lara Nux ships a privileged Ubuntu daemon plus desktop assets for local
 Laravel development with managed DNS, HTTPS, PHP runtime switching, and
 service orchestration.`) + "\n"
	if err := os.WriteFile(filepath.Join(debianDir, "control"), []byte(control), 0o644); err != nil {
		t.Fatalf("write control file: %v", err)
	}

	debPath := filepath.Join(workspace, "lara-nux_0.1.0~beta1-1_amd64.deb")
	runCommand(t, context.Background(), repoRoot, "dpkg-deb", "--build", packageRoot, debPath)
	if _, err := os.Stat(debPath); err != nil {
		t.Fatalf("expected deb at %s: %v", debPath, err)
	}
	return debPath
}

func inspectContainerIP(t *testing.T, ctx context.Context, repoRoot string, containerName string) string {
	t.Helper()
	ip := strings.TrimSpace(runCommand(t, ctx, repoRoot, "docker", "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", containerName))
	if ip == "" {
		t.Fatal("container has no IPv4 address")
	}
	return ip
}

func runCommand(t *testing.T, ctx context.Context, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %s %s\n%v\n%s", name, strings.Join(args, " "), err, string(output))
	}
	return string(output)
}

func runCommandEnv(t *testing.T, ctx context.Context, dir string, env map[string]string, name string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %s %s\n%v\n%s", name, strings.Join(args, " "), err, string(output))
	}
	return string(output)
}

func tryRunHost(ctx context.Context, dir string, name string, args ...string) string {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	output, _ := cmd.CombinedOutput()
	return string(output)
}

func copyFile(t *testing.T, source string, destination string) {
	t.Helper()
	payload, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read %s: %v", source, err)
	}
	if err := os.WriteFile(destination, payload, 0o644); err != nil {
		t.Fatalf("write %s: %v", destination, err)
	}
}

func decodeJSONBody(t *testing.T, payload []byte, target any) {
	t.Helper()
	if err := json.Unmarshal(payload, target); err != nil {
		t.Fatalf("decode json payload: %v\n%s", err, string(payload))
	}
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
