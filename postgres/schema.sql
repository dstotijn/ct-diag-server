CREATE TABLE diagnosis_keys
(
    key bytea NOT NULL,
    interval_number bigint NOT NULL, -- We don't really need 64 bytes, but uint32's range doesn't fit in `integer`
    CONSTRAINT diagnosis_keys_pkey PRIMARY KEY (key)
)