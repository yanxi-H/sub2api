import type { ComposerTranslation } from 'vue-i18n'
import type { MonitorCenterStatus } from '@/api/admin/monitorCenter'

export const STATUS_ORDER: Record<MonitorCenterStatus, number> = {
  operational: 0,
  unknown: -1,
  under_maintenance: 2,
  degraded_performance: 3,
  partial_outage: 4,
  major_outage: 5,
}

export const STATUS_COLORS: Record<MonitorCenterStatus, string> = {
  operational: '#4f9d73',
  degraded_performance: '#c98524',
  partial_outage: '#d46645',
  major_outage: '#c84d49',
  under_maintenance: '#71879d',
  unknown: '#a3a3a8',
}

export function statusTone(status?: MonitorCenterStatus | null): string {
  if (status === 'operational') return 'good'
  if (status === 'degraded_performance' || status === 'under_maintenance') return 'warn'
  if (status === 'partial_outage' || status === 'major_outage') return 'bad'
  return 'unknown'
}

export function statusLabel(t: ComposerTranslation, status?: MonitorCenterStatus | null): string {
  return t(`admin.monitorCenter.status.${status || 'unknown'}`)
}

export function formatNumber(value?: number | null, maximumFractionDigits = 1): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  return new Intl.NumberFormat(undefined, { maximumFractionDigits }).format(value)
}

export function formatCompactNumber(value?: number | null): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  return new Intl.NumberFormat(undefined, { notation: 'compact', maximumFractionDigits: 1 }).format(value)
}

export function formatPercent(value?: number | null, digits = 2): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  return `${value.toFixed(digits)}%`
}

export function formatMs(value?: number | null): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  if (value >= 10_000) return `${(value / 1000).toFixed(value >= 100_000 ? 0 : 1)} s`
  return `${new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 }).format(value)} ms`
}

export function formatDateTime(value?: string | null): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString(undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  })
}

export function formatAxisTime(value: string, includeDate = false): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString(undefined, includeDate
    ? { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }
    : { hour: '2-digit', minute: '2-digit', hour12: false })
}

export function chartPalette() {
  const dark = document.documentElement.classList.contains('dark')
  return {
    dark,
    text: dark ? '#a3a3a8' : '#6e6e73',
    title: dark ? '#f5f5f7' : '#1d1d1f',
    grid: dark ? 'rgba(255,255,255,.08)' : 'rgba(60,60,67,.09)',
    tooltipBg: dark ? '#242426' : '#ffffff',
    tooltipBorder: dark ? '#3a3a3c' : '#e5e5ea',
  }
}

export function nullableMetric(value?: number | null): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}
