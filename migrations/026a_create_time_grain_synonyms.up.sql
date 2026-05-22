-- Create ai_time_grains table to store customizable time grain synonyms and suffixes.
CREATE TABLE IF NOT EXISTS ai_time_grains (
    grain          TEXT PRIMARY KEY,
    suffix         TEXT NOT NULL,
    requires_time  BOOLEAN NOT NULL DEFAULT FALSE,
    synonyms       TEXT[] NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
