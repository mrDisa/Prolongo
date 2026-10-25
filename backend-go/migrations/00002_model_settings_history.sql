-- +goose Up
-- Every saved version of the model settings, so an earlier calculation can be opened again.
-- core.model_settings stays the single current row; this table only grows.
CREATE TABLE core.model_settings_history (
    version bigint PRIMARY KEY,
    data jsonb NOT NULL,
    saved_at timestamptz NOT NULL
);

-- The version that is current when the migration runs starts the history.
INSERT INTO core.model_settings_history (version, data, saved_at)
SELECT version, data, updated_at FROM core.model_settings;

-- +goose Down
DROP TABLE core.model_settings_history;
