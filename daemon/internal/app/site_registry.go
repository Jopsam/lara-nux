package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidLaravelPath = errors.New("invalid laravel path")
	ErrDuplicateSiteName  = errors.New("duplicate site name")
	ErrDuplicateDomain    = errors.New("duplicate site domain")
	ErrSiteNotFound       = errors.New("site not found")
	ErrInvalidDomain      = errors.New("invalid site domain")
)

type SiteStatus string

const (
	SiteStatusReady    SiteStatus = "ready"
	SiteStatusDegraded SiteStatus = "degraded"
	SiteStatusConflict SiteStatus = "conflict"
)

const siteTLSModeAuto = "auto"

type SiteRecord struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	RootPath      string     `json:"rootPath"`
	Domain        string     `json:"domain"`
	PHPVersion    string     `json:"phpVersion"`
	TLS           string     `json:"tls"`
	Status        SiteStatus `json:"status"`
	StatusMessage string     `json:"statusMessage,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	LastCheckedAt time.Time  `json:"lastCheckedAt,omitempty"`
}

type SiteRegistrationInput struct {
	RootPath   string `json:"rootPath"`
	Domain     string `json:"domain,omitempty"`
	PHPVersion string `json:"phpVersion"`
}

type SiteRegistry struct {
	path  string
	clock func() time.Time
	mu    sync.Mutex
}

type siteRegistryState struct {
	Sites []SiteRecord `json:"sites"`
}

var domainLabelPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)

func NewSiteRegistry(path string) *SiteRegistry {
	return &SiteRegistry{
		path:  path,
		clock: func() time.Time { return time.Now().UTC() },
	}
}

func NewSiteRegistryFromPaths(paths AppPaths) *SiteRegistry {
	return NewSiteRegistry(filepath.Join(paths.StateDir, "sites.json"))
}

func (r *SiteRegistry) Register(ctx context.Context, input SiteRegistrationInput) (SiteRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	select {
	case <-ctx.Done():
		return SiteRecord{}, ctx.Err()
	default:
	}

	rootPath, err := normalizeRootPath(input.RootPath)
	if err != nil {
		return SiteRecord{}, err
	}

	if err := ValidateLaravelPath(rootPath); err != nil {
		return SiteRecord{}, err
	}

	state, err := r.load()
	if err != nil {
		return SiteRecord{}, err
	}

	name := deriveSiteName(rootPath)
	domain, err := normalizeDomain(input.Domain, name)
	if err != nil {
		return SiteRecord{}, err
	}

	for _, existing := range state.Sites {
		if strings.EqualFold(existing.Name, name) {
			return SiteRecord{}, fmt.Errorf("%w: site name %q is already registered", ErrDuplicateSiteName, name)
		}

		if strings.EqualFold(existing.Domain, domain) {
			return SiteRecord{}, fmt.Errorf("%w: domain %q is already registered", ErrDuplicateDomain, domain)
		}
	}

	now := r.clock()
	record := SiteRecord{
		ID:            newRecordID(),
		Name:          name,
		RootPath:      rootPath,
		Domain:        domain,
		PHPVersion:    NormalizePHPVersion(input.PHPVersion),
		TLS:           siteTLSModeAuto,
		Status:        SiteStatusReady,
		StatusMessage: "Site registered and ready for orchestration.",
		CreatedAt:     now,
		UpdatedAt:     now,
		LastCheckedAt: now,
	}

	state.Sites = append(state.Sites, record)
	if err := r.save(state); err != nil {
		return SiteRecord{}, err
	}

	return record, nil
}

func (r *SiteRegistry) List(ctx context.Context) ([]SiteRecord, error) {
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

	sites := append([]SiteRecord(nil), state.Sites...)
	sort.Slice(sites, func(i, j int) bool {
		return sites[i].Domain < sites[j].Domain
	})

	return sites, nil
}

func (r *SiteRegistry) Get(ctx context.Context, siteID string) (SiteRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	select {
	case <-ctx.Done():
		return SiteRecord{}, ctx.Err()
	default:
	}

	state, err := r.load()
	if err != nil {
		return SiteRecord{}, err
	}

	for _, site := range state.Sites {
		if site.ID == siteID {
			return site, nil
		}
	}

	return SiteRecord{}, fmt.Errorf("%w: %s", ErrSiteNotFound, siteID)
}

func (r *SiteRegistry) Update(ctx context.Context, record SiteRecord) (SiteRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	select {
	case <-ctx.Done():
		return SiteRecord{}, ctx.Err()
	default:
	}

	state, err := r.load()
	if err != nil {
		return SiteRecord{}, err
	}

	for index := range state.Sites {
		if state.Sites[index].ID != record.ID {
			continue
		}

		record.CreatedAt = state.Sites[index].CreatedAt
		record.UpdatedAt = r.clock()
		state.Sites[index] = record

		if err := r.save(state); err != nil {
			return SiteRecord{}, err
		}

		return record, nil
	}

	return SiteRecord{}, fmt.Errorf("%w: %s", ErrSiteNotFound, record.ID)
}

func (r *SiteRegistry) Delete(ctx context.Context, siteID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	state, err := r.load()
	if err != nil {
		return err
	}

	for index := range state.Sites {
		if state.Sites[index].ID != siteID {
			continue
		}

		state.Sites = append(state.Sites[:index], state.Sites[index+1:]...)
		return r.save(state)
	}

	return fmt.Errorf("%w: %s", ErrSiteNotFound, siteID)
}

func (r *SiteRegistry) SetStatus(ctx context.Context, siteID string, status SiteStatus, message string, checkedAt time.Time) (SiteRecord, error) {
	record, err := r.Get(ctx, siteID)
	if err != nil {
		return SiteRecord{}, err
	}

	record.Status = status
	record.StatusMessage = strings.TrimSpace(message)
	record.LastCheckedAt = checkedAt.UTC()

	return r.Update(ctx, record)
}

func (r *SiteRegistry) load() (siteRegistryState, error) {
	var state siteRegistryState
	if err := loadStateFile(r.path, &state); err != nil {
		return state, err
	}

	if state.Sites == nil {
		state.Sites = []SiteRecord{}
	}

	return state, nil
}

func (r *SiteRegistry) save(state siteRegistryState) error {
	return saveStateFile(r.path, state, 0o640)
}

func ValidateLaravelPath(rootPath string) error {
	info, err := os.Stat(rootPath)
	if err != nil {
		return fmt.Errorf("%w: inspect project root: %v", ErrInvalidLaravelPath, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("%w: %s is not a directory", ErrInvalidLaravelPath, rootPath)
	}

	for _, required := range []string{"artisan", filepath.Join("public", "index.php"), "composer.json"} {
		requiredPath := filepath.Join(rootPath, required)
		fileInfo, statErr := os.Stat(requiredPath)
		if statErr != nil {
			return fmt.Errorf("%w: missing %s", ErrInvalidLaravelPath, required)
		}

		if fileInfo.IsDir() {
			return fmt.Errorf("%w: expected file at %s", ErrInvalidLaravelPath, required)
		}
	}

	return nil
}

func normalizeRootPath(rootPath string) (string, error) {
	trimmed := strings.TrimSpace(rootPath)
	if trimmed == "" {
		return "", fmt.Errorf("%w: rootPath is required", ErrInvalidLaravelPath)
	}

	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("%w: resolve absolute path: %v", ErrInvalidLaravelPath, err)
	}

	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		return resolvedPath, nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return absPath, nil
	}

	return "", fmt.Errorf("%w: resolve symlinks: %v", ErrInvalidLaravelPath, err)
}

func normalizeDomain(raw string, fallbackName string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		value = fallbackName + ".test"
	}

	if !strings.HasSuffix(value, ".test") {
		return "", fmt.Errorf("%w: domain %q must end with .test", ErrInvalidDomain, value)
	}

	host := strings.TrimSuffix(value, ".test")
	if host == "" {
		return "", fmt.Errorf("%w: domain %q must include a hostname label", ErrInvalidDomain, value)
	}

	for _, label := range strings.Split(host, ".") {
		if !domainLabelPattern.MatchString(label) {
			return "", fmt.Errorf("%w: label %q is not a valid local hostname segment", ErrInvalidDomain, label)
		}
	}

	return value, nil
}

func deriveSiteName(rootPath string) string {
	base := filepath.Base(rootPath)
	base = strings.ToLower(strings.TrimSpace(base))
	replacer := strings.NewReplacer("_", "-", " ", "-", ".", "-")
	base = replacer.Replace(base)

	var builder strings.Builder
	lastDash := false
	for _, char := range base {
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9':
			builder.WriteRune(char)
			lastDash = false
		case char == '-':
			if builder.Len() > 0 && !lastDash {
				builder.WriteRune('-')
				lastDash = true
			}
		}
	}

	name := strings.Trim(builder.String(), "-")
	if name == "" {
		return "site"
	}

	return name
}

func newRecordID() string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("site-%d", time.Now().UTC().UnixNano())
	}

	return "site-" + hex.EncodeToString(buffer)
}
