package api

import "net/http"

func registerHealthRoutes(mux *http.ServeMux, deps RouterDependencies) {
	mux.HandleFunc("/rpc/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET /rpc/health")
			return
		}

		report, err := deps.HealthService.Report(r.Context())
		if err != nil {
			handleAppError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, report)
	})
}
