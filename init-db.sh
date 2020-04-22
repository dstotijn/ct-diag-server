#!/bin/bash
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE TABLE diagnosis_keys
    (
        key uuid NOT NULL,
        day_number integer NOT NULL,
        CONSTRAINT diagnosis_keys_pkey PRIMARY KEY (key)
    )
EOSQL