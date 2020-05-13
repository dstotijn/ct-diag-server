CREATE TABLE diagnosis_keys
(
    temporary_exposure_key bytea NOT NULL,
    rolling_start_number bigint NOT NULL, -- We don't really need 64 bytes, but uint32's range doesn't fit in `integer`
    rolling_period bigint NOT NULL, -- We don't really need 64 bytes, but uint32's range doesn't fit in `integer`
    transmission_risk_level bytea NOT NULL,
    uploaded_at timestamp with time zone NOT NULL,
    index bigserial NOT NULL UNIQUE,
    CONSTRAINT diagnosis_keys_pkey PRIMARY KEY (temporary_exposure_key)
);

CREATE INDEX index_idx
    ON diagnosis_keys USING btree
    (index ASC);