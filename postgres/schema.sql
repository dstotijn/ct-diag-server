CREATE TABLE diagnosis_keys
(
    key bytea NOT NULL,
    interval_number bigint NOT NULL, -- We don't really need 64 bytes, but uint32's range doesn't fit in `integer`
    created_at timestamp without time zone NOT NULL,
    CONSTRAINT diagnosis_keys_pkey PRIMARY KEY (key)
);

CREATE INDEX created_at_idx
    ON diagnosis_keys USING btree
    (created_at ASC NULLS LAST);