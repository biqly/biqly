CREATE TABLE drift_reports (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  model_id        UUID NOT NULL REFERENCES semantic_models(id) ON DELETE CASCADE,
  datasource_id   UUID NOT NULL REFERENCES datasources(id) ON DELETE CASCADE,
  sync_event_id   UUID REFERENCES audit_events(id) ON DELETE SET NULL,
  severity        TEXT NOT NULL DEFAULT 'info',
  drifts          JSONB NOT NULL DEFAULT '[]',
  resolved        BOOLEAN NOT NULL DEFAULT false,
  resolved_by     TEXT,
  resolved_at     TIMESTAMPTZ,
  detected_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_drift_reports_model ON drift_reports(model_id);
CREATE INDEX idx_drift_reports_unresolved ON drift_reports(resolved) WHERE NOT resolved;
