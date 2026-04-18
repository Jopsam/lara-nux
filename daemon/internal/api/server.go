package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jopsam/lara-nux/daemon/internal/app"
	"github.com/jopsam/lara-nux/daemon/internal/host"
)

type RouterDependencies struct {
	HealthService         *app.HealthService
	PHPManager            *app.PHPManager
	ServiceManager        *app.ServiceManager
	SiteActivationService *app.SiteActivationService
	SiteManagementService *app.SiteManagementService
	RuntimeOnboarding     *app.RuntimeOnboardingService
	RuntimeCatalogService *app.RuntimeCatalogService
	ResolverManager       host.ResolverManager
}

type rpcError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type rpcEnvelope struct {
	OK    bool      `json:"ok"`
	Data  any       `json:"data,omitempty"`
	Error *rpcError `json:"error,omitempty"`
}

func NewRouter(deps RouterDependencies) http.Handler {
	mux := http.NewServeMux()
	if deps.SiteActivationService == nil {
		panic("api: SiteActivationService is required")
	}
	if deps.RuntimeOnboarding == nil {
		panic("api: RuntimeOnboarding is required")
	}
	if deps.SiteManagementService == nil || deps.RuntimeCatalogService == nil {
		panic("api: query services are required")
	}
	if deps.HealthService == nil || deps.PHPManager == nil || deps.ServiceManager == nil {
		panic("api: core services are required")
	}
	registerHealthRoutes(mux, deps)
	registerSiteRoutes(mux, deps)
	registerPHPRoutes(mux, deps)
	registerServiceRoutes(mux, deps)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "RPC endpoint not found")
	})
	return mux
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(rpcEnvelope{OK: true, Data: data})
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(rpcEnvelope{OK: false, Error: &rpcError{Code: code, Message: message}})
}

func handleAppError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, app.ErrInvalidLaravelPath), errors.Is(err, app.ErrInvalidDomain), errors.Is(err, app.ErrUnsupportedRuntime), errors.Is(err, app.ErrUnverifiablePHP), errors.Is(err, app.ErrInvalidServiceAction):
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, app.ErrDuplicateDomain), errors.Is(err, app.ErrDuplicateSiteName):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, host.ErrResolverConflict), errors.Is(err, host.ErrActivationValidation), errors.Is(err, host.ErrPackageVerification):
		writeError(w, http.StatusConflict, "host_conflict", err.Error())
	case errors.Is(err, host.ErrUnsupportedPackage):
		writeError(w, http.StatusBadRequest, "unsupported_package", err.Error())
	case errors.Is(err, app.ErrSiteNotFound), errors.Is(err, app.ErrRuntimeNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
	}
}
