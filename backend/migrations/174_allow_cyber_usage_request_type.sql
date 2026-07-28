-- Allow cyber_policy usage rows to persist request_type=4.
--
-- Code has RequestTypeCyberBlocked=4, but older databases still have the
-- usage_logs_request_type_check constraint from migration 061, which only
-- allows 0..3. Replace the constraint idempotently so cyber hits do not fail
-- usage log writes. NOT VALID keeps the constraint add cheap on large tables;
-- validation scans existing rows with a lighter lock.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE usage_logs
    DROP CONSTRAINT IF EXISTS usage_logs_request_type_check;

ALTER TABLE usage_logs
    ADD CONSTRAINT usage_logs_request_type_check
    CHECK (request_type IN (0, 1, 2, 3, 4)) NOT VALID;

ALTER TABLE usage_logs
    VALIDATE CONSTRAINT usage_logs_request_type_check;

COMMENT ON COLUMN usage_logs.request_type IS 'Request type enum: 0=unknown, 1=sync, 2=stream, 3=ws_v2, 4=cyber.';
COMMENT ON COLUMN ops_error_logs.request_type IS 'Request type enum: 0=unknown, 1=sync, 2=stream, 3=ws_v2, 4=cyber. Matches usage_logs.request_type semantics.';
