-- Project logs are best-effort, privacy-limited operational telemetry, not audit facts.
CREATE TABLE platform_project_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id uuid,
  created_at timestamptz NOT NULL,
  level varchar(8) NOT NULL CHECK (level IN ('DEBUG','INFO','WARN','ERROR')),
  event varchar(80) NOT NULL,
  domain varchar(80) NOT NULL DEFAULT '',
  operation varchar(160) NOT NULL DEFAULT '',
  error_code varchar(100) NOT NULL DEFAULT '',
  request_id uuid,
  trace_id char(32),
  status integer CHECK (status BETWEEN 100 AND 599),
  duration_ms bigint CHECK (duration_ms >= 0)
);
CREATE INDEX platform_project_logs_tenant_page ON platform_project_logs (tenant_id, created_at DESC, id DESC);
CREATE INDEX platform_project_logs_expiry ON platform_project_logs (created_at);
COMMENT ON TABLE platform_project_logs IS 'Best-effort allowlisted slog projection only; 90-day rolling read window and bounded background deletion; NULL tenant visible only to global super_admin';
