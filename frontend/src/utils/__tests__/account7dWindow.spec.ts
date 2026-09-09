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

  it('reads a future Zhipu coding plan weekly reset', () => {
    const resetAt = '2026-08-24T00:00:00.000Z'
    const result = getAccount7dReset({
      platform: 'zhipu',
      extra: { zhipu_weekly_reset_at: resetAt }
    }, now)

    expect(result).toEqual({ resetAt, source: 'zhipu_weekly' })
    expect(account7dResetSourceI18nKey('zhipu_weekly')).toBe('keys.sync7dWindowSourceZhipu')
  })

  it('reads a future Kimi coding plan weekly reset and ignores expired ones', () => {
    const resetAt = '2026-08-25T00:00:00.000Z'
    expect(getAccount7dReset({ platform: 'kimi', extra: { kimi_weekly_reset_at: resetAt } }, now))
      .toEqual({ resetAt, source: 'kimi_weekly' })
    expect(account7dResetSourceI18nKey('kimi_weekly')).toBe('keys.sync7dWindowSourceKimi')

    expect(getAccount7dReset({ platform: 'kimi', extra: { kimi_weekly_reset_at: '2026-08-10T00:00:00.000Z' } }, now)).toBeNull()
  })

  it('ignores CN snapshots left over from a different provider', () => {
    // 账号从 zhipu 切到 kimi 端点后，extra 里残留的 zhipu 快照不得成为窗口来源。
    expect(getAccount7dReset({
      platform: 'kimi',
      extra: { zhipu_weekly_reset_at: '2026-08-24T00:00:00.000Z' }
    }, now)).toBeNull()

    expect(getAccount7dReset({
      platform: 'zhipu',
      extra: { kimi_weekly_reset_at: '2026-08-25T00:00:00.000Z' }
    }, now)).toBeNull()

    // 非 CN 平台一律不读 CN 快照。
    expect(getAccount7dReset({
      platform: 'openai',
      extra: { zhipu_weekly_reset_at: '2026-08-24T00:00:00.000Z' }
    }, now)).toBeNull()
  })
})
