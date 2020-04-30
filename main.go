package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/dstotijn/ct-diag-server/api"
	"github.com/dstotijn/ct-diag-server/diag"
	"github.com/dstotijn/ct-diag-server/postgres"
)

func main() {
	ctx := context.Background()

	var addr string
	flag.StringVar(&addr, "addr", ":80", "HTTP listen address")
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

	// Use in-memory cache for serving Diagnosis Keys.
	cache := &diag.MemoryCache{}

	handler, err := api.NewHandler(ctx, db, cache)
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
