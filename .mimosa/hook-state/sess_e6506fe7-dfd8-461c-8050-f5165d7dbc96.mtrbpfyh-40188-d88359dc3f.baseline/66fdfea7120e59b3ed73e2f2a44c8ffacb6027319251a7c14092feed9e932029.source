import type { Account } from '@/types'

export type Account7dResetSource = 'codex' | 'passive' | 'weekly_quota' | 'grok_weekly'

export interface Account7dReset {
  resetAt: string
  source: Account7dResetSource
}

function parseResetAtValue(value: unknown): string | null {
  if (typeof value === 'string' && value.trim()) {
    const date = new Date(value)
    return Number.isNaN(date.getTime()) ? null : date.toISOString()
  }
  if (typeof value === 'number' && Number.isFinite(value) && value > 0) {
    const millis = value > 1e12 ? value : value * 1000
    const date = new Date(millis)
    return Number.isNaN(date.getTime()) ? null : date.toISOString()
  }
  return null
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null
}

function grokWeeklyResetAt(extra: Record<string, unknown>): string | null {
  const billing = asRecord(extra.grok_billing_snapshot)
  if (!billing) return null
  const periodType = typeof billing.period_type === 'string' ? billing.period_type.trim().toLowerCase() : ''
  const hasWeeklyUsage = typeof billing.usage_percent === 'number'
  if (periodType !== 'weekly' && !hasWeeklyUsage) return null
  return parseResetAtValue(billing.period_end)
}

export function getAccount7dReset(account: Pick<Account, 'extra'> | null | undefined, now = Date.now()): Account7dReset | null {
  const extra = account?.extra || {}
  const candidates: Array<{ resetAt: string | null; source: Account7dResetSource }> = [
    { resetAt: parseResetAtValue(extra.codex_7d_reset_at), source: 'codex' },
    { resetAt: parseResetAtValue(extra.passive_usage_7d_reset), source: 'passive' },
    { resetAt: parseResetAtValue(extra.quota_weekly_reset_at), source: 'weekly_quota' },
    { resetAt: grokWeeklyResetAt(extra), source: 'grok_weekly' }
  ]
  const match = candidates.find((candidate) => {
    if (!candidate.resetAt) return false
    return new Date(candidate.resetAt).getTime() > now
  })
  return match?.resetAt ? { resetAt: match.resetAt, source: match.source } : null
}

export function getAccount7dResetAt(account: Pick<Account, 'extra'> | null | undefined, now = Date.now()): string | null {
  return getAccount7dReset(account, now)?.resetAt ?? null
}

export function account7dResetSourceI18nKey(source: Account7dResetSource): string {
  switch (source) {
    case 'codex':
      return 'keys.sync7dWindowSourceCodex'
    case 'passive':
      return 'keys.sync7dWindowSourcePassive'
    case 'weekly_quota':
      return 'keys.sync7dWindowSourceWeeklyQuota'
    case 'grok_weekly':
      return 'keys.sync7dWindowSourceGrok'
    default:
      return 'common.unknown'
  }
}
