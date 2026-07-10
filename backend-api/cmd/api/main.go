package main

import (
	"log"
	"net/http"

	"qiyuan/backend-api/internal/app"
	"qiyuan/backend-api/internal/config"
)

func main() {
	cfg := config.Load()
	server := app.NewServer()

	log.Printf("backend-api listening on %s", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, server.Handler()); err != nil {
		log.Fatal(err)
	}
}
