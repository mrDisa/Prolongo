-- +goose Up
CREATE SCHEMA IF NOT EXISTS core;

CREATE TABLE core.users (
    id bigserial PRIMARY KEY,
    username text NOT NULL UNIQUE,
    display_name text NOT NULL DEFAULT '',
    password_hash text,                 -- null for OIDC users
    oidc_subject text UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE core.sessions (
    token_hash bytea PRIMARY KEY,       -- sha256(token); the token itself lives only in the cookie
    user_id bigint NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,
    auth_method text NOT NULL CHECK (auth_method IN ('password', 'oidc')),
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);
CREATE INDEX sessions_expires_idx ON core.sessions (expires_at);

CREATE TABLE core.model_settings (
    id smallint PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    version bigint NOT NULL,
    data jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE core.risk_daily (
    day date PRIMARY KEY,
    customers_high int NOT NULL,
    customers_medium int NOT NULL,
    customers_low int NOT NULL,
    customers_none int NOT NULL,
    services_at_risk int NOT NULL,
    expected_loss_cents bigint NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE core.risk_daily;
DROP TABLE core.model_settings;
DROP TABLE core.sessions;
DROP TABLE core.users;
-- The schema itself stays: it also holds goose_db_version.
