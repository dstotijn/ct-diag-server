package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/dstotijn/ct-diag-server/api"
	"github.com/dstotijn/ct-diag-server/db/postgres"
	"github.com/dstotijn/ct-diag-server/diag"
)

func main() {
	ctx := context.Background()

	var (
		addr               string
		maxUploadBatchSize uint
	)
	flag.StringVar(&addr, "addr", ":80", "HTTP listen address")
	flag.UintVar(&maxUploadBatchSize, "maxUploadBatchSize", 14, "Maximum upload batch size")
	flag.Parse()

	db, err := postgres.New(mustGetEnv("POSTGRES_DSN"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	cfg := diag.Config{
		Repository:         db,
		Cache:              &diag.MemoryCache{},
		MaxUploadBatchSize: maxUploadBatchSize,
	}
	handler, err := api.NewHandler(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}

	// Start the HTTP server.
	log.Printf("Server listening on %v ...\n", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("environment variable `%v` cannot be empty", key)
	}
	return v
}
