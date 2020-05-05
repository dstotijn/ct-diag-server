package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dstotijn/ct-diag-server/api"
	"github.com/dstotijn/ct-diag-server/db/postgres"
	"github.com/dstotijn/ct-diag-server/diag"

	"go.uber.org/zap"
)

func main() {
	ctx := context.Background()

	var (
		addr               string
		maxUploadBatchSize uint
		isDev              bool
		cacheInterval      time.Duration
	)
	flag.StringVar(&addr, "addr", ":80", "HTTP listen address")
	flag.UintVar(&maxUploadBatchSize, "maxUploadBatchSize", 14, "Maximum upload batch size")
	flag.BoolVar(&isDev, "dev", false, "Boolean indicating whether the app is running in a dev environment")
	flag.DurationVar(&cacheInterval, "cacheInterval", 5*time.Minute, "Interval between cache refresh")
	flag.Parse()

	logger, err := newLogger(isDev)
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Sync()
	zap.RedirectStdLog(logger)

	db, err := postgres.New(mustGetEnv("POSTGRES_DSN"))
	if err != nil {
		logger.Fatal("Could not create PostgreSQL client.", zap.Error(err))
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		logger.Fatal("Could not connect to database.", zap.Error(err))
	}

	cfg := diag.Config{
		Repository:         db,
		Cache:              &diag.MemoryCache{},
		CacheInterval:      cacheInterval,
		MaxUploadBatchSize: maxUploadBatchSize,
		Logger:             logger,
	}
	handler, err := api.NewHandler(ctx, cfg, logger)
	if err != nil {
		logger.Fatal("Could not create HTTP handler.", zap.Error(err))
	}

	// Start the HTTP server.
	logger.Info("Server started.", zap.String("addr", addr))
	if err := http.ListenAndServe(addr, handler); err != nil {
		logger.Fatal("Server stopped.", zap.Error(err))
	}
}

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("Environment variable `%s` cannot be empty.", key)
	}
	return v
}

func newLogger(isDev bool) (*zap.Logger, error) {
	if isDev {
		return zap.NewDevelopment()
	}
	return zap.NewProduction()
}
