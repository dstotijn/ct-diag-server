#!/usr/bin/env bash
set -Eeuo pipefail

DSN=$1
INTERVAL=$2

psql -v ON_ERROR_STOP=1 $DSN <<-EOSQL
    DELETE FROM diagnosis_keys
    WHERE created_at < CURRENT_TIME - interval '$INTERVAL';
EOSQL