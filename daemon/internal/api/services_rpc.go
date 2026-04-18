package api

import (
	"net/http"

	"github.com/jopsam/lara-nux/daemon/internal/app"
)

type serviceActionRequest struct {
	Service string `json:"service"`
	Action  string `json:"action"`
}

func registerServiceRoutes(mux *http.ServeMux, deps RouterDependencies) {
	mux.HandleFunc("/rpc/services.action", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST /rpc/services.action")
			return
		}

		var request serviceActionRequest
		if err := decodeJSON(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}

		status, err := deps.ServiceManager.Action(r.Context(), request.Service, app.ServiceAction(request.Action))
		if err != nil {
			handleAppError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, status)
	})
}
