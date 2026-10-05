CREATE INDEX idx_jobs_status_scheduled_at
ON jobs(status, scheduled_at);