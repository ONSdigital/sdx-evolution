package main

import "net/http"

// HealthcheckHandler responds to a healthcheck request with the current
// health of the service.
func HealthcheckHandler(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte(`{"status":"ok"}`))
	return
}
