package main

import (
	"log"
	"net/http"

	"github.com/Lucy-97/browser-agent/backend-api/internal/app"
	"github.com/Lucy-97/browser-agent/backend-api/internal/config"
)

func main() {
	cfg := config.Load()
	server := app.NewServer()

	log.Printf("backend-api listening on %s", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, server.Handler()); err != nil {
		log.Fatal(err)
	}
}
