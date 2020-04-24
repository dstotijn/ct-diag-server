CREATE TABLE diagnosis_keys
(
    key bytea NOT NULL,
    day_number integer NOT NULL,
    CONSTRAINT diagnosis_keys_pkey PRIMARY KEY (key)
)