package main

import (
	"log"
	"net/http"

	"github.com/tnnz20/youthpreneur-be/internal/config"
)

func main() {
	cfg := config.Load()
	mux := config.Bootstrap()

	log.Printf("listening on %s", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, mux); err != nil {
		log.Fatal(err)
	}
}
