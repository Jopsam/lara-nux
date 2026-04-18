package api

import (
	"net/http"

	"github.com/jopsam/lara-nux/daemon/internal/app"
)

type registerSiteRequest struct {
	RootPath   string `json:"rootPath"`
	Domain     string `json:"domain,omitempty"`
	PHPVersion string `json:"phpVersion,omitempty"`
}

type updateSiteRequest struct {
	SiteID     string  `json:"siteId"`
	RootPath   *string `json:"rootPath,omitempty"`
	Domain     *string `json:"domain,omitempty"`
	PHPVersion *string `json:"phpVersion,omitempty"`
}

func registerSiteRoutes(mux *http.ServeMux, deps RouterDependencies) {
	mux.HandleFunc("/rpc/sites.list", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET /rpc/sites.list")
			return
		}

		sites, err := deps.SiteManagementService.List(r.Context())
		if err != nil {
			handleAppError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, sites)
	})

	mux.HandleFunc("/rpc/sites.get", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET /rpc/sites.get?siteId=<id>")
			return
		}

		siteID := r.URL.Query().Get("siteId")
		site, err := deps.SiteManagementService.Get(r.Context(), siteID)
		if err != nil {
			handleAppError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, site)
	})

	mux.HandleFunc("/rpc/sites.register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST /rpc/sites.register")
			return
		}

		var request registerSiteRequest
		if err := decodeJSON(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		result, err := deps.SiteActivationService.Activate(r.Context(), app.SiteRegistrationInput{
			RootPath:   request.RootPath,
			Domain:     request.Domain,
			PHPVersion: request.PHPVersion,
		})
		if err != nil {
			handleAppError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, result)
	})

	mux.HandleFunc("/rpc/sites.update", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST /rpc/sites.update")
			return
		}

		var request updateSiteRequest
		if err := decodeJSON(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		record, err := deps.SiteManagementService.Update(r.Context(), app.SiteUpdateInput{
			SiteID:     request.SiteID,
			RootPath:   request.RootPath,
			Domain:     request.Domain,
			PHPVersion: request.PHPVersion,
		})
		if err != nil {
			handleAppError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, record)
	})
}
