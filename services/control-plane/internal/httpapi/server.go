package httpapi

import (
	"encoding/json"
	"net/http"
)

const serviceName = "buildplane-control-plane"

// NewServer builds the HTTP surface for the control-plane process.
func NewServer(version string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", getOnly(healthz))
	mux.HandleFunc("/readyz", getOnly(readyz))
	mux.HandleFunc("/version", getOnly(versionHandler(version)))
	return mux
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func readyz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ready",
	})
}

func versionHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"name":    serviceName,
			"version": version,
		})
	}
}

func getOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed\n", http.StatusMethodNotAllowed)
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
