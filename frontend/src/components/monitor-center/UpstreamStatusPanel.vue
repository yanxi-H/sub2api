<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { AlertTriangle, CheckCircle2, Cloud, ExternalLink } from '@lucide/vue'
import type {
  MonitorCenterIncident,
  MonitorCenterOpenAIHistoryResponse,
  MonitorCenterOpenAIStatusResponse,
  MonitorCenterServiceGroup,
  MonitorCenterStatus,
} from '@/api/admin/monitorCenter'
import { formatDateTime, statusLabel, statusTone } from './monitorCenterUtils'

const props = defineProps<{
  status: MonitorCenterOpenAIStatusResponse | null
  history: MonitorCenterOpenAIHistoryResponse | null
  rangeLabel: string
  loading: boolean
}>()

const { t } = useI18n()
const groupOrder = ['api', 'chatgpt', 'codex']
const groups = computed(() => [...(props.status?.groups ?? [])].sort((a, b) => groupOrder.indexOf(a.key) - groupOrder.indexOf(b.key)))
const latestIncident = computed(() => props.status?.incidents?.[0] ?? null)
const historySlotCount = 30

interface HistorySlot {
  start: Date
  end: Date
  status: MonitorCenterStatus | null
  incidentCount: number
  fetchFailed: boolean
}

function groupStatusAt(point: MonitorCenterOpenAIHistoryResponse['points'][number], key: string): MonitorCenterStatus {
  if (key === 'api') return point.api_status
  if (key === 'chatgpt') return point.chatgpt_status
  if (key === 'codex') return point.codex_status
  return 'unknown'
}

function statusSeverity(status: MonitorCenterStatus): number {
  if (status === 'operational') return 0
  if (status === 'under_maintenance') return 1
  if (status === 'degraded_performance') return 2
  if (status === 'partial_outage') return 3
  if (status === 'major_outage') return 4
  return -1
}

function historySlots(group: MonitorCenterServiceGroup): HistorySlot[] {
  const start = new Date(props.history?.start_time ?? '')
  const end = new Date(props.history?.end_time ?? '')
  const duration = end.getTime() - start.getTime()
  if (!Number.isFinite(duration) || duration <= 0) return []
  const slotDuration = duration / historySlotCount
  const slots = Array.from({ length: historySlotCount }, (_, index): HistorySlot => ({
    start: new Date(start.getTime() + index * slotDuration),
    end: new Date(start.getTime() + (index + 1) * slotDuration),
    status: null,
    incidentCount: 0,
    fetchFailed: false,
  }))
  for (const point of props.history?.points ?? []) {
    const timestamp = new Date(point.timestamp).getTime()
    if (!Number.isFinite(timestamp) || timestamp < start.getTime() || timestamp > end.getTime()) continue
    const index = Math.min(historySlotCount - 1, Math.floor((timestamp - start.getTime()) / slotDuration))
    const slot = slots[index]
    slot.incidentCount = Math.max(slot.incidentCount, point.active_incident_count || 0)
    if (point.fetch_status !== 'success') {
      slot.fetchFailed = true
      slot.status = 'unknown'
      continue
    }
    if (slot.fetchFailed) continue
    const next = groupStatusAt(point, group.key)
    if (slot.status == null || statusSeverity(next) > statusSeverity(slot.status)) slot.status = next
  }
  return slots
}

const historyCoverage = computed(() => {
  const start = new Date(props.history?.start_time ?? '').getTime()
  const end = new Date(props.history?.end_time ?? '').getTime()
  const duration = end - start
  const expected = Number.isFinite(duration) && duration > 0 ? Math.ceil(duration / 60_000) : 0
  const minutes = new Set((props.history?.points ?? []).map(point => Math.floor(new Date(point.timestamp).getTime() / 60_000)).filter(Number.isFinite))
  const actual = Math.min(expected, minutes.size)
  return { actual, expected, percent: expected ? Math.round(actual / expected * 100) : 0 }
})

function slotTitle(slot: HistorySlot): string {
  const time = `${formatDateTime(slot.start.toISOString())} - ${formatDateTime(slot.end.toISOString())}`
  const state = slot.status == null ? t('admin.monitorCenter.upstream.missingSample') : statusLabel(t, slot.status)
  const incident = slot.incidentCount ? ` · ${t('admin.monitorCenter.upstream.incidentMarker', { count: slot.incidentCount })}` : ''
  return `${time} · ${state}${incident}`
}

function incidentTone(incident: MonitorCenterIncident): string {
  if (incident.impact === 'critical' || incident.impact === 'major') return 'bad'
  if (incident.impact === 'minor' || incident.status !== 'resolved') return 'warn'
  return 'good'
}
</script>

<template>
  <article class="mc-panel mc-panel-pad mc-upstream" :class="{ 'mc-loading': loading && !status }">
    <div class="mc-upstream-head">
      <div class="mc-status-row">
        <div class="mc-icon-tile"><Cloud /></div>
        <div class="mc-upstream-copy">
          <div class="mc-panel-title">{{ t('admin.monitorCenter.upstream.title') }}</div>
          <div class="mc-panel-subtitle">{{ status?.overall_description || t('admin.monitorCenter.common.unknown') }}</div>
        </div>
      </div>
      <div class="mc-upstream-meta">
        <span class="mc-badge" :class="statusTone(status?.overall_status)">{{ statusLabel(t, status?.overall_status) }}</span>
        <div>{{ t('admin.monitorCenter.upstream.incidentCount', { count: status?.incidents?.length ?? 0 }) }} · {{ t('admin.monitorCenter.upstream.lastSync', { time: formatDateTime(status?.last_success_at) }) }}</div>
      </div>
    </div>

    <div v-if="status?.stale || status?.fetch_status === 'failed'" class="mc-notice mc-stale">
      <AlertTriangle />
      <span>{{ status?.last_success_at ? t('admin.monitorCenter.upstream.stale', { time: formatDateTime(status.last_success_at) }) : t('admin.monitorCenter.upstream.unavailable') }}</span>
    </div>

    <div v-if="groups.length" class="mc-groups">
      <section v-for="group in groups" :key="group.key" class="mc-group">
        <div class="mc-group-head">
          <strong>{{ group.name }}</strong>
          <span :class="`mc-${statusTone(group.status)}`">{{ statusLabel(t, group.status) }}</span>
        </div>
        <div class="mc-component-list">
          <div v-for="component in group.components" :key="component.key">
            <span>{{ component.name }}</span>
            <span :class="`mc-${statusTone(component.status)}`">
              {{ component.matched ? statusLabel(t, component.status) : t('admin.monitorCenter.upstream.notReported') }}
            </span>
          </div>
        </div>
      </section>
    </div>
    <div v-else class="mc-empty mc-upstream-empty">{{ loading ? t('common.loading') : t('common.noData') }}</div>

    <div class="mc-bands">
      <div class="mc-band-summary">
        <span>{{ t('admin.monitorCenter.upstream.rangeHistory', { range: rangeLabel }) }}</span>
        <span>{{ t('admin.monitorCenter.upstream.coverage', historyCoverage) }}</span>
      </div>
      <div v-for="group in groups" :key="group.key" class="mc-band-row">
        <span>{{ group.name }}</span>
        <div v-if="historySlots(group).length" class="mc-band" :aria-label="`${group.name} ${t('admin.monitorCenter.upstream.rangeHistory', { range: rangeLabel })}`">
          <i
            v-for="(item, index) in historySlots(group)"
            :key="index"
            :class="[item.status == null ? 'missing' : statusTone(item.status), { incident: item.incidentCount > 0 }]"
            :title="slotTitle(item)"
          />
        </div>
        <div v-else class="mc-band-empty">{{ t('common.noData') }}</div>
      </div>
      <div class="mc-band-legend">
        <span><i class="good" />{{ t('admin.monitorCenter.status.operational') }}</span>
        <span><i class="incident" />{{ t('admin.monitorCenter.upstream.unresolvedIncident') }}</span>
        <span><i class="missing" />{{ t('admin.monitorCenter.upstream.missingSample') }}</span>
      </div>
    </div>

    <div class="mc-incident" :class="latestIncident ? incidentTone(latestIncident) : 'good'">
      <component :is="latestIncident ? AlertTriangle : CheckCircle2" />
      <div v-if="latestIncident" class="mc-incident-copy">
        <strong>{{ latestIncident.name }}</strong>
        <span>{{ t('admin.monitorCenter.upstream.incidentMeta', { status: latestIncident.status, impact: latestIncident.impact, time: formatDateTime(latestIncident.updated_at) }) }}</span>
        <span v-if="latestIncident.affected_components.length">{{ latestIncident.affected_components.join(' · ') }}</span>
        <p v-if="latestIncident.latest_update?.body">{{ latestIncident.latest_update.body }}</p>
      </div>
      <div v-else class="mc-incident-copy">
        <strong>{{ t('admin.monitorCenter.upstream.noIncidents') }}</strong>
        <span>{{ t('admin.monitorCenter.upstream.noIncidentsHint') }}</span>
      </div>
      <a href="https://status.openai.com" target="_blank" rel="noopener noreferrer" :title="t('admin.monitorCenter.upstream.openOfficial')">
        <ExternalLink />
        <span>{{ t('admin.monitorCenter.upstream.officialDetails') }}</span>
      </a>
    </div>
  </article>
</template>

<style scoped>
.mc-upstream { min-width: 0; }
.mc-upstream-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.mc-upstream-copy { min-width: 0; }
.mc-upstream-meta { flex: none; color: var(--mc-subtle); font-size: 9px; text-align: right; }
.mc-upstream-meta > div { margin-top: 6px; }
.mc-stale { display: flex; align-items: center; gap: 7px; margin-top: 10px; }
.mc-stale svg { width: 14px; height: 14px; flex: none; }
.mc-groups { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 7px; margin-top: 11px; }
.mc-group { min-width: 0; border: 1px solid color-mix(in srgb, var(--mc-line) 72%, transparent); border-radius: 7px; padding: 10px; background: var(--mc-soft); }
.mc-group-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; font-size: 11px; }
.mc-group-head > span { font-size: 9px; font-weight: 700; }
.mc-component-list { display: grid; gap: 5px; margin-top: 8px; }
.mc-component-list > div { display: flex; align-items: center; justify-content: space-between; gap: 8px; color: var(--mc-muted); font-size: 9px; }
.mc-component-list > div span:first-child { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mc-component-list > div span:last-child { flex: none; font-weight: 650; }
.mc-upstream-empty { min-height: 94px; }
.mc-bands { display: grid; gap: 6px; margin-top: 11px; border-top: 1px solid var(--mc-line); padding-top: 10px; }
.mc-band-summary { display: flex; align-items: center; justify-content: space-between; gap: 10px; color: var(--mc-subtle); font-size: 9px; }
.mc-band-summary span:first-child { color: var(--mc-muted); font-weight: 650; }
.mc-band-row { display: grid; grid-template-columns: 56px minmax(0, 1fr); align-items: center; gap: 8px; }
.mc-band-row > span { color: var(--mc-muted); font-size: 9px; font-weight: 650; }
.mc-band { display: grid; grid-auto-flow: column; grid-auto-columns: minmax(3px, 1fr); gap: 2px; height: 9px; }
.mc-band i { display: block; min-width: 0; border-radius: 2px; background: var(--mc-subtle); }
.mc-band i.good { background: var(--mc-green); }
.mc-band i.warn { background: var(--mc-orange); }
.mc-band i.bad { background: var(--mc-red); }
.mc-band i.missing { background: color-mix(in srgb, var(--mc-subtle) 35%, transparent); }
.mc-band i.incident { box-shadow: inset 0 -3px var(--mc-orange); }
.mc-band-legend { display: flex; justify-content: flex-end; gap: 12px; color: var(--mc-subtle); font-size: 8px; }
.mc-band-legend span { display: inline-flex; align-items: center; gap: 4px; }
.mc-band-legend i { width: 8px; height: 8px; border-radius: 2px; background: var(--mc-green); }
.mc-band-legend i.incident { background: linear-gradient(to bottom, var(--mc-green) 0 60%, var(--mc-orange) 60%); }
.mc-band-legend i.missing { background: color-mix(in srgb, var(--mc-subtle) 35%, transparent); }
.mc-band-empty { color: var(--mc-subtle); font-size: 9px; }
.mc-incident { display: grid; grid-template-columns: 16px minmax(0, 1fr) auto; align-items: start; gap: 9px; margin-top: 11px; border-radius: 7px; padding: 10px; background: var(--mc-soft); }
.mc-incident > svg { width: 15px; height: 15px; margin-top: 1px; }
.mc-incident.good > svg { color: var(--mc-green); }
.mc-incident.warn > svg { color: var(--mc-orange); }
.mc-incident.bad > svg { color: var(--mc-red); }
.mc-incident-copy { min-width: 0; }
.mc-incident-copy strong { display: block; font-size: 10px; line-height: 1.35; }
.mc-incident-copy span { display: block; margin-top: 3px; color: var(--mc-subtle); font-size: 9px; line-height: 1.4; }
.mc-incident-copy p { display: -webkit-box; overflow: hidden; margin: 6px 0 0; color: var(--mc-muted); font-size: 9px; line-height: 1.45; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
.mc-incident a { display: inline-flex; align-items: center; gap: 4px; color: var(--mc-blue); font-size: 9px; white-space: nowrap; }
.mc-incident a svg { width: 12px; height: 12px; }
@media (max-width: 760px) {
  .mc-upstream-head { flex-direction: column; }
  .mc-upstream-meta { text-align: left; }
  .mc-groups { grid-template-columns: 1fr; }
  .mc-incident { grid-template-columns: 16px minmax(0, 1fr); }
  .mc-incident a { grid-column: 2; }
}
</style>
