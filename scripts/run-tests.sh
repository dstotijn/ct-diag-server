#!/usr/bin/env bash
set -Eeuo pipefail

NAME="pg-ct-diag-test"
PORT="54321"

export POSTGRES_DSN="postgres://$NAME:$NAME@127.0.0.1:$PORT/$NAME?sslmode=disable"

function clean {
  docker rm -f $NAME 
}

trap clean EXIT

docker run --rm --name $NAME -d -p $PORT:5432 \
  -e POSTGRES_USER=$NAME \
  -e POSTGRES_PASSWORD=$NAME \
  -v $PWD/postgres/schema.sql:/docker-entrypoint-initdb.d/schema.sql \
  postgres:11.7-alpine

go test ./... -v -count=1
