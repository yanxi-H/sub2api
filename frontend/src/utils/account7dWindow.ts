import type { Account } from '@/types'

export type Account7dResetSource = 'codex' | 'passive' | 'zhipu_weekly' | 'kimi_weekly' | 'weekly_quota' | 'grok_weekly'

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

// 国产 coding plan（zhipu/kimi）周窗口：CNProviderQuotaService 探测落盘的
// {provider}_weekly_reset_at（RFC3339）。
function cnWeeklyResetAt(extra: Record<string, unknown>, provider: 'zhipu' | 'kimi'): string | null {
  return parseResetAtValue(extra[`${provider}_weekly_reset_at`])
}

export function getAccount7dReset(
  account: Pick<Account, 'extra' | 'platform'> | null | undefined,
  now = Date.now()
): Account7dReset | null {
  const extra = account?.extra || {}
  const platform = account?.platform
  // 国产 coding plan 周窗口按当前账号平台过滤：账号切换供应商后，旧供应商的
  // 快照仍会残留在 extra 中（后端不做统一清理），直接扫描会把旧值当成当前
  // 窗口来源，甚至给出后端会拒绝的同步选项。
  const zhipuWeekly = platform === 'zhipu' ? cnWeeklyResetAt(extra, 'zhipu') : null
  const kimiWeekly = platform === 'kimi' ? cnWeeklyResetAt(extra, 'kimi') : null
  const candidates: Array<{ resetAt: string | null; source: Account7dResetSource }> = [
    { resetAt: parseResetAtValue(extra.codex_7d_reset_at), source: 'codex' },
    { resetAt: parseResetAtValue(extra.passive_usage_7d_reset), source: 'passive' },
    { resetAt: zhipuWeekly, source: 'zhipu_weekly' },
    { resetAt: kimiWeekly, source: 'kimi_weekly' },
    { resetAt: parseResetAtValue(extra.quota_weekly_reset_at), source: 'weekly_quota' },
    { resetAt: grokWeeklyResetAt(extra), source: 'grok_weekly' }
  ]
  const match = candidates.find((candidate) => {
    if (!candidate.resetAt) return false
    return new Date(candidate.resetAt).getTime() > now
  })
  return match?.resetAt ? { resetAt: match.resetAt, source: match.source } : null
}

export function getAccount7dResetAt(
  account: Pick<Account, 'extra' | 'platform'> | null | undefined,
  now = Date.now()
): string | null {
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
    case 'zhipu_weekly':
      return 'keys.sync7dWindowSourceZhipu'
    case 'kimi_weekly':
      return 'keys.sync7dWindowSourceKimi'
    default:
      return 'common.unknown'
  }
}
