package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"service": "go-api",
			"status":  "ok",
		})
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"message": "AI OnCall Go API bootstrap is ready",
		})
	})

	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:    "0.0.0.0:" + port,
		Handler: mux,
	}

	log.Fatal(server.ListenAndServe())
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
