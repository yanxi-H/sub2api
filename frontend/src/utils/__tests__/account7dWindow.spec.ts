import { describe, expect, it } from 'vitest'
import { account7dResetSourceI18nKey, getAccount7dReset, getAccount7dResetAt } from '../account7dWindow'

describe('account7dWindow', () => {
  const now = Date.parse('2026-08-19T12:00:00.000Z')

  it('reads a future Grok weekly billing period end', () => {
    const resetAt = '2026-08-22T08:00:00.000Z'
    const result = getAccount7dReset({
      extra: {
        grok_billing_snapshot: {
          period_type: 'weekly',
          usage_percent: 41,
          period_end: resetAt
        }
      }
    }, now)

    expect(result).toEqual({ resetAt, source: 'grok_weekly' })
    expect(getAccount7dResetAt({ extra: { grok_billing_snapshot: { period_type: 'weekly', period_end: resetAt } } }, now)).toBe(resetAt)
    expect(account7dResetSourceI18nKey('grok_weekly')).toBe('keys.sync7dWindowSourceGrok')
  })

  it('ignores Grok monthly period ends and expired weekly windows', () => {
    expect(getAccount7dReset({
      extra: {
        grok_billing_snapshot: {
          period_type: 'monthly',
          period_end: '2026-09-01T00:00:00.000Z'
        }
      }
    }, now)).toBeNull()

    expect(getAccount7dReset({
      extra: {
        grok_billing_snapshot: {
          period_type: 'weekly',
          period_end: '2026-08-18T00:00:00.000Z'
        }
      }
    }, now)).toBeNull()
  })

  it('still prefers an explicit Codex window when both exist', () => {
    const result = getAccount7dReset({
      extra: {
        codex_7d_reset_at: '2026-08-21T00:00:00.000Z',
        grok_billing_snapshot: {
          period_type: 'weekly',
          period_end: '2026-08-22T00:00:00.000Z'
        }
      }
    }, now)

    expect(result).toEqual({
      resetAt: '2026-08-21T00:00:00.000Z',
      source: 'codex'
    })
  })
})
