package api

import (
	"net/http"

	"github.com/jopsam/lara-nux/daemon/internal/app"
)

type switchPHPRequest struct {
	SiteID     string `json:"siteId"`
	PHPVersion string `json:"phpVersion"`
}

type registerPHPRequest struct {
	Version    string `json:"version,omitempty"`
	BinaryPath string `json:"binaryPath,omitempty"`
	FPMService string `json:"fpmService,omitempty"`
	Source     string `json:"source,omitempty"`
	PackageKey string `json:"packageKey,omitempty"`
}

type setDefaultPHPRequest struct {
	Version string `json:"version"`
}

func registerPHPRoutes(mux *http.ServeMux, deps RouterDependencies) {
	mux.HandleFunc("/rpc/php.list", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET /rpc/php.list")
			return
		}

		runtimes, err := deps.RuntimeCatalogService.ListRegistered(r.Context())
		if err != nil {
			handleAppError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, runtimes)
	})

	mux.HandleFunc("/rpc/php.default", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			runtime, err := deps.RuntimeCatalogService.DefaultRuntime(r.Context())
			if err != nil {
				handleAppError(w, err)
				return
			}
			if runtime == nil {
				writeJSON(w, http.StatusOK, map[string]any{"runtime": nil})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"runtime": runtime})
		case http.MethodPost:
			var request setDefaultPHPRequest
			if err := decodeJSON(r, &request); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
				return
			}

			runtime, err := deps.RuntimeCatalogService.SetDefault(r.Context(), request.Version)
			if err != nil {
				handleAppError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, map[string]any{"runtime": runtime})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET or POST /rpc/php.default")
		}
	})

	mux.HandleFunc("/rpc/php.inventory", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET /rpc/php.inventory")
			return
		}

		catalog, err := deps.RuntimeCatalogService.Inventory(r.Context())
		if err != nil {
			handleAppError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, catalog)
	})

	mux.HandleFunc("/rpc/php.register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST /rpc/php.register")
			return
		}

		var request registerPHPRequest
		if err := decodeJSON(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		result, err := deps.RuntimeOnboarding.Register(r.Context(), app.RuntimeRegistrationRequest{
			Version:    request.Version,
			BinaryPath: request.BinaryPath,
			FPMService: request.FPMService,
			Source:     request.Source,
			PackageKey: request.PackageKey,
		})
		if err != nil {
			handleAppError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, result)
	})

	mux.HandleFunc("/rpc/php.switch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST /rpc/php.switch")
			return
		}

		var request switchPHPRequest
		if err := decodeJSON(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		record, err := deps.PHPManager.SwitchSiteRuntime(r.Context(), request.SiteID, request.PHPVersion)
		if err != nil {
			handleAppError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, record)
	})
}
