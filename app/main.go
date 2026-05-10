package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("GET /echo/{str}", func(w http.ResponseWriter, r *http.Request) {
		str := r.PathValue("str")
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(str))
	})

	server := &http.Server{
		Addr:    "0.0.0.0:4221",
		Handler: mux,
	}

	fmt.Println("Server listening on 0.0.0.0:4221")
	if err := server.ListenAndServe(); err != nil {
		fmt.Println("Failed to start server:", err)
		os.Exit(1)
	}
}
