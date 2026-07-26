-- A 57 MB ordinary request routed through the old recovery fallback can exhaust
-- a 512 MiB gateway container because validation and forwarding retain multiple
-- views of the payload.
-- Reset policies created with the former 64 MB default to the conservative
-- 3 / 20 / 32 MB defaults. Runtime validation enforces the same ceiling.
UPDATE accounts
SET extra = jsonb_set(
        jsonb_set(
            jsonb_set(
                COALESCE(extra, '{}'::jsonb),
                '{request_body_normal_limit_bytes}',
                '3145728'::jsonb,
                true
            ),
            '{request_body_heavy_limit_bytes}',
            '20971520'::jsonb,
            true
        ),
        '{request_body_recovery_limit_bytes}',
        '33554432'::jsonb,
        true
    )
WHERE platform = 'openai'
  AND COALESCE(extra, '{}'::jsonb)->>'request_body_admission_enabled' = 'true'
  AND (
      (
          jsonb_typeof(COALESCE(extra, '{}'::jsonb)->'request_body_recovery_limit_bytes') = 'number'
          AND (extra->>'request_body_recovery_limit_bytes')::numeric > 33554432
      )
      OR (
          jsonb_typeof(COALESCE(extra, '{}'::jsonb)->'request_body_recovery_limit_bytes') = 'string'
          AND extra->>'request_body_recovery_limit_bytes' ~ '^[0-9]+([.][0-9]+)?$'
          AND (extra->>'request_body_recovery_limit_bytes')::numeric > 33554432
      )
  );
