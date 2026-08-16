package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"buildingops/internal/app"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	address := ":" + port
	server := &http.Server{Addr: address, Handler: app.NewServer(app.Options{})}
	log.Printf("building operations console listening on http://localhost:%s", port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
