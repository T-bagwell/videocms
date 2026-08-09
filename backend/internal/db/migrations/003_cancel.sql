ALTER TABLE libraries DROP CONSTRAINT libraries_scan_status_check;
ALTER TABLE libraries ADD CONSTRAINT libraries_scan_status_check
    CHECK (scan_status IN ('idle', 'scanning', 'error', 'cancelled'));

