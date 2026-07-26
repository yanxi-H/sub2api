-- Replace the legacy per-account hard body limit and compact bypass with the
-- tiered OpenAI request-body admission policy. Existing explicit policy values
-- win over these defaults so the migration is safe for mixed-version writes.
WITH legacy_accounts AS (
    SELECT
        id,
        CASE
            WHEN jsonb_typeof(COALESCE(extra, '{}'::jsonb)->'request_body_limit_bytes') = 'number'
                THEN (extra->>'request_body_limit_bytes')::numeric
            WHEN jsonb_typeof(COALESCE(extra, '{}'::jsonb)->'request_body_limit_bytes') = 'string'
                AND extra->>'request_body_limit_bytes' ~ '^[0-9]+([.][0-9]+)?$'
                THEN (extra->>'request_body_limit_bytes')::numeric
            ELSE NULL
        END AS legacy_limit
    FROM accounts
    WHERE platform = 'openai'
      AND (
          COALESCE(extra, '{}'::jsonb) ? 'request_body_limit_bytes'
          OR COALESCE(extra, '{}'::jsonb) ? 'allow_compact_request_body_limit_bypass'
      )
)
UPDATE accounts AS account
SET extra = CASE
        WHEN legacy.legacy_limit > 0 THEN jsonb_build_object(
            'request_body_admission_enabled', true,
            'request_body_normal_limit_bytes', 3145728,
            'request_body_heavy_limit_bytes', 20971520,
            'request_body_recovery_limit_bytes', 67108864
        )
        ELSE '{}'::jsonb
    END
    || (
        COALESCE(account.extra, '{}'::jsonb)
        - 'request_body_limit_bytes'
        - 'allow_compact_request_body_limit_bypass'
    )
FROM legacy_accounts AS legacy
WHERE account.id = legacy.id;

-- The old hard limit was accepted for other account platforms too. It has no
-- replacement outside OpenAI Responses, so remove the now-dead fields there.
UPDATE accounts
SET extra = COALESCE(extra, '{}'::jsonb)
    - 'request_body_limit_bytes'
    - 'allow_compact_request_body_limit_bypass'
WHERE platform IS DISTINCT FROM 'openai'
  AND (
      COALESCE(extra, '{}'::jsonb) ? 'request_body_limit_bytes'
      OR COALESCE(extra, '{}'::jsonb) ? 'allow_compact_request_body_limit_bypass'
  );
