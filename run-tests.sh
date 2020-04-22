#!/bin/bash
set -e

POSTGRES_DSN=postgres://ct-diag-test:ct-diag-test@127.0.0.1:54321/ct-diag-test?sslmode=disable 

docker-compose -f docker-compose.test.yml up -d
go test ./... -v
docker-compose -f docker-compose.test.yml stop postgres-test