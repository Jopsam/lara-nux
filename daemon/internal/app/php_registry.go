package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrUnsupportedRuntime = errors.New("unsupported php runtime")
	ErrRuntimeNotFound    = errors.New("php runtime not found")
	ErrUnverifiablePHP    = errors.New("unverifiable php runtime")
)

var supportedPHPVersions = map[string]struct{}{
	"8.1": {},
	"8.2": {},
	"8.3": {},
	"8.4": {},
}

type PHPRuntimeRecord struct {
	Version      string    `json:"version"`
	BinaryPath   string    `json:"binaryPath"`
	FPMService   string    `json:"fpmService"`
	Source       string    `json:"source,omitempty"`
	RegisteredAt time.Time `json:"registeredAt"`
}

type RuntimeRegistrationInput struct {
	Version    string `json:"version"`
	BinaryPath string `json:"binaryPath"`
	FPMService string `json:"fpmService,omitempty"`
	Source     string `json:"source,omitempty"`
}

type PHPRegistry struct {
	path  string
	clock func() time.Time
	run   func(ctx context.Context, binaryPath string, args ...string) (string, error)
	mu    sync.Mutex
}

type phpRegistryState struct {
	DefaultVersion string             `json:"defaultVersion,omitempty"`
	Runtimes       []PHPRuntimeRecord `json:"runtimes"`
}

func NewPHPRegistry(path string) *PHPRegistry {
	return &PHPRegistry{
		path:  path,
		clock: func() time.Time { return time.Now().UTC() },
		run:   runPHPBinary,
	}
}

func NewPHPRegistryFromPaths(paths AppPaths) *PHPRegistry {
	return NewPHPRegistry(filepath.Join(paths.StateDir, "php-runtimes.json"))
}

func (r *PHPRegistry) Register(ctx context.Context, input RuntimeRegistrationInput) (PHPRuntimeRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	select {
	case <-ctx.Done():
		return PHPRuntimeRecord{}, ctx.Err()
	default:
	}

	binaryPath, err := normalizeExecutablePath(input.BinaryPath)
	if err != nil {
		return PHPRuntimeRecord{}, err
	}

	resolvedVersion, err := r.verifyRuntime(ctx, NormalizePHPVersion(input.Version), binaryPath)
	if err != nil {
		return PHPRuntimeRecord{}, err
	}

	serviceName := strings.TrimSpace(input.FPMService)
	if serviceName == "" {
		serviceName = PHPFPMServiceName(resolvedVersion)
	}

	state, err := r.load()
	if err != nil {
		return PHPRuntimeRecord{}, err
	}

	record := PHPRuntimeRecord{
		Version:      resolvedVersion,
		BinaryPath:   binaryPath,
		FPMService:   serviceName,
		Source:       strings.TrimSpace(input.Source),
		RegisteredAt: r.clock(),
	}

	replaced := false
	for index := range state.Runtimes {
		if state.Runtimes[index].Version == resolvedVersion {
			state.Runtimes[index] = record
			replaced = true
			break
		}
	}

	if !replaced {
		state.Runtimes = append(state.Runtimes, record)
	}

	if state.DefaultVersion == "" {
		state.DefaultVersion = resolvedVersion
	}

	sort.Slice(state.Runtimes, func(i, j int) bool {
		return state.Runtimes[i].Version < state.Runtimes[j].Version
	})

	if err := r.save(state); err != nil {
		return PHPRuntimeRecord{}, err
	}

	return record, nil
}

func (r *PHPRegistry) verifyRuntime(ctx context.Context, requestedVersion string, binaryPath string) (string, error) {
	actualVersion, err := r.detectRuntimeVersion(ctx, binaryPath)
	if err != nil {
		return "", err
	}

	if !r.IsSupported(actualVersion) {
		return "", fmt.Errorf("%w: detected PHP %s at %s is outside the supported matrix", ErrUnsupportedRuntime, actualVersion, binaryPath)
	}

	if requestedVersion != "" && requestedVersion != actualVersion {
		return "", fmt.Errorf("%w: binary %s reports PHP %s but %s was requested", ErrUnverifiablePHP, binaryPath, actualVersion, requestedVersion)
	}

	return actualVersion, nil
}

func (r *PHPRegistry) detectRuntimeVersion(ctx context.Context, binaryPath string) (string, error) {
	output, err := r.run(ctx, binaryPath, "-r", `echo PHP_MAJOR_VERSION, ".", PHP_MINOR_VERSION;`)
	if err != nil {
		return "", fmt.Errorf("%w: inspect php version from %s: %v (%s)", ErrUnverifiablePHP, binaryPath, err, strings.TrimSpace(output))
	}

	version := NormalizePHPVersion(output)
	if version == "" {
		return "", fmt.Errorf("%w: php binary %s returned an empty version", ErrUnverifiablePHP, binaryPath)
	}

	return version, nil
}

func (r *PHPRegistry) List(ctx context.Context) ([]PHPRuntimeRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	state, err := r.load()
	if err != nil {
		return nil, err
	}

	runtimes := append([]PHPRuntimeRecord(nil), state.Runtimes...)
	return runtimes, nil
}

func (r *PHPRegistry) Get(ctx context.Context, version string) (PHPRuntimeRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	select {
	case <-ctx.Done():
		return PHPRuntimeRecord{}, ctx.Err()
	default:
	}

	state, err := r.load()
	if err != nil {
		return PHPRuntimeRecord{}, err
	}

	normalized := NormalizePHPVersion(version)
	for _, runtime := range state.Runtimes {
		if runtime.Version == normalized {
			return runtime, nil
		}
	}

	return PHPRuntimeRecord{}, fmt.Errorf("%w: %s", ErrRuntimeNotFound, normalized)
}

func (r *PHPRegistry) DefaultRuntime(ctx context.Context) (PHPRuntimeRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	select {
	case <-ctx.Done():
		return PHPRuntimeRecord{}, ctx.Err()
	default:
	}

	state, err := r.load()
	if err != nil {
		return PHPRuntimeRecord{}, err
	}

	if state.DefaultVersion == "" {
		return PHPRuntimeRecord{}, fmt.Errorf("%w: no default runtime configured", ErrRuntimeNotFound)
	}

	for _, runtime := range state.Runtimes {
		if runtime.Version == state.DefaultVersion {
			return runtime, nil
		}
	}

	return PHPRuntimeRecord{}, fmt.Errorf("%w: default runtime %s", ErrRuntimeNotFound, state.DefaultVersion)
}

func (r *PHPRegistry) SetDefault(ctx context.Context, version string) (PHPRuntimeRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	select {
	case <-ctx.Done():
		return PHPRuntimeRecord{}, ctx.Err()
	default:
	}

	state, err := r.load()
	if err != nil {
		return PHPRuntimeRecord{}, err
	}

	normalized := NormalizePHPVersion(version)
	for _, runtime := range state.Runtimes {
		if runtime.Version == normalized {
			state.DefaultVersion = normalized
			if err := r.save(state); err != nil {
				return PHPRuntimeRecord{}, err
			}

			return runtime, nil
		}
	}

	return PHPRuntimeRecord{}, fmt.Errorf("%w: %s", ErrRuntimeNotFound, normalized)
}

func (r *PHPRegistry) IsSupported(version string) bool {
	_, ok := supportedPHPVersions[NormalizePHPVersion(version)]
	return ok
}

func (r *PHPRegistry) load() (phpRegistryState, error) {
	var state phpRegistryState
	if err := loadStateFile(r.path, &state); err != nil {
		return state, err
	}

	if state.Runtimes == nil {
		state.Runtimes = []PHPRuntimeRecord{}
	}

	return state, nil
}

func (r *PHPRegistry) save(state phpRegistryState) error {
	return saveStateFile(r.path, state, 0o640)
}

func NormalizePHPVersion(raw string) string {
	version := strings.TrimSpace(strings.ToLower(raw))
	version = strings.TrimPrefix(version, "php")
	return strings.TrimSpace(version)
}

func PHPFPMServiceName(version string) string {
	return "php" + NormalizePHPVersion(version) + "-fpm"
}

func normalizeExecutablePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("%w: php binary path is required", ErrUnverifiablePHP)
	}

	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("%w: resolve binary path: %v", ErrUnverifiablePHP, err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("%w: inspect binary path: %v", ErrUnverifiablePHP, err)
	}

	if info.IsDir() {
		return "", fmt.Errorf("%w: %s is a directory", ErrUnverifiablePHP, absPath)
	}

	if info.Mode()&0o111 == 0 {
		return "", fmt.Errorf("%w: %s is not executable", ErrUnverifiablePHP, absPath)
	}

	return absPath, nil
}

func runPHPBinary(ctx context.Context, binaryPath string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, binaryPath, args...)
	output, err := command.CombinedOutput()
	return string(output), err
}
