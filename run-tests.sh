#!/bin/bash
set -e

NAME="pg-ct-diag-test"
PORT="54321"

export POSTGRES_DSN="postgres://$NAME:$NAME@127.0.0.1:$PORT/$NAME?sslmode=disable"

docker run --rm --name $NAME -d -p $PORT:5432 \
  -e POSTGRES_USER=$NAME \
  -e POSTGRES_PASSWORD=$NAME \
  -v $PWD/postgres/schema.sql:/docker-entrypoint-initdb.d/schema.sql \
  postgres:11.7-alpine

go test ./... -count=1

docker rm -f $NAME 