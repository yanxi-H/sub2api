<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { AlertTriangle, CheckCircle2, ChevronRight, Cloud, ExternalLink } from '@lucide/vue'
import type {
  MonitorCenterIncident,
  MonitorCenterOpenAIHistoryPoint,
  MonitorCenterOpenAIHistoryResponse,
  MonitorCenterOpenAIStatusResponse,
  MonitorCenterServiceGroup,
  MonitorCenterStatus,
} from '@/api/admin/monitorCenter'
import { STATUS_COLORS, STATUS_ORDER, formatDateTime, formatPercent, statusLabel } from './monitorCenterUtils'

const props = defineProps<{
  status: MonitorCenterOpenAIStatusResponse | null
  history: MonitorCenterOpenAIHistoryResponse | null
  rangeLabel: string
  loading: boolean
}>()

const { t } = useI18n()
const groupOrder = ['api', 'chatgpt', 'codex']
const groups = computed(() => [...(props.status?.groups ?? [])].sort((a, b) => groupOrder.indexOf(a.key) - groupOrder.indexOf(b.key)))
const incidents = computed(() => props.history?.incidents ?? [])
const selectedIncidentID = ref('')
watch(incidents, (value) => {
  if (!value.some(item => item.id === selectedIncidentID.value)) selectedIncidentID.value = value[0]?.id ?? ''
}, { immediate: true })
const selectedIncident = computed(() => incidents.value.find(item => item.id === selectedIncidentID.value) ?? null)
const historySlotCount = 36

interface HistorySlot {
  start: Date
  end: Date
  status: MonitorCenterStatus | null
  fetchFailed: boolean
  samples: number
  latencyMs: number | null
  failureReason: string
  incidentIDs: string[]
}

function groupStatusAt(point: MonitorCenterOpenAIHistoryPoint, key: string): MonitorCenterStatus {
  if (key === 'api') return point.api_status
  if (key === 'chatgpt') return point.chatgpt_status
  if (key === 'codex') return point.codex_status
  return 'unknown'
}

function statusSeverity(status: MonitorCenterStatus): number {
  return STATUS_ORDER[status]
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
    fetchFailed: false,
    samples: 0,
    latencyMs: null,
    failureReason: '',
    incidentIDs: [],
  }))
  for (const point of props.history?.points ?? []) {
    const timestamp = new Date(point.timestamp).getTime()
    if (!Number.isFinite(timestamp) || timestamp < start.getTime() || timestamp > end.getTime()) continue
    const index = Math.min(historySlotCount - 1, Math.floor((timestamp - start.getTime()) / slotDuration))
    const slot = slots[index]
    slot.samples += 1
    if (slot.latencyMs == null || point.latency_ms > slot.latencyMs) slot.latencyMs = point.latency_ms
    slot.incidentIDs = [...new Set([...slot.incidentIDs, ...(point.incident_refs?.[group.key] ?? [])])]
    if (point.fetch_status !== 'success') {
      slot.fetchFailed = true
      if (slot.status == null) slot.status = 'unknown'
      slot.failureReason = point.failure_reason ?? ''
      continue
    }
    const next = groupStatusAt(point, group.key)
    if (slot.status == null || statusSeverity(next) > statusSeverity(slot.status)) slot.status = next
  }
  return slots
}

const historyCoverage = computed(() => {
  const start = new Date(props.history?.start_time ?? '').getTime()
  const end = new Date(props.history?.end_time ?? '').getTime()
  const expected = Number.isFinite(end - start) && end > start ? Math.ceil((end - start) / 60_000) : 0
  const minutes = new Set((props.history?.points ?? []).map(point => Math.floor(new Date(point.timestamp).getTime() / 60_000)).filter(Number.isFinite))
  const actual = Math.min(expected, minutes.size)
  return { actual, expected, percent: expected ? Math.round(actual / expected * 100) : 0 }
})

function slotTitle(slot: HistorySlot): string {
  const time = `${formatDateTime(slot.start.toISOString())} - ${formatDateTime(slot.end.toISOString())}`
  const state = slot.status == null ? t('admin.monitorCenter.upstream.missingSample') : statusLabel(t, slot.status)
  const details = [t('admin.monitorCenter.upstream.sampleWindow', { count: slot.samples }), slot.latencyMs == null ? '' : `${slot.latencyMs} ms`, slot.failureReason]
  if (slot.incidentIDs.length) details.push(t('admin.monitorCenter.upstream.linkedIncidents', { count: slot.incidentIDs.length }))
  return `${time} · ${state} · ${details.filter(Boolean).join(' · ')}`
}

function incidentTone(incident: MonitorCenterIncident): string {
  if (incident.status === 'resolved') return 'good'
  if (incident.impact === 'critical' || incident.impact === 'major') return 'bad'
  return 'warn'
}

function incidentStart(incident: MonitorCenterIncident): string | null {
  return incident.started_at ?? incident.created_at ?? incident.updates[incident.updates.length - 1]?.updated_at ?? null
}

function incidentDuration(incident: MonitorCenterIncident): string {
  const start = new Date(incidentStart(incident) ?? '').getTime()
  const end = incident.status === 'resolved'
    ? new Date(incident.resolved_at ?? incident.updated_at).getTime()
    : Date.now()
  if (!Number.isFinite(start) || !Number.isFinite(end) || end < start) return '-'
  const minutes = Math.max(1, Math.round((end - start) / 60_000))
  if (minutes < 60) return t('admin.monitorCenter.upstream.durationMinutes', { count: minutes })
  const hours = Math.round(minutes / 6) / 10
  return t('admin.monitorCenter.upstream.durationHours', { count: hours })
}

function incidentURL(incident: MonitorCenterIncident): string {
  return incident.url || 'https://status.openai.com'
}
</script>

<template>
  <article class="mc-panel mc-panel-pad mc-upstream" :class="{ 'mc-loading': loading && !status }">
    <div class="mc-upstream-head">
      <div class="mc-status-row">
        <div class="mc-icon-tile"><Cloud /></div>
        <div class="mc-upstream-copy">
          <div class="mc-panel-title">{{ t('admin.monitorCenter.upstream.title') }}</div>
          <div class="mc-panel-subtitle">{{ t('admin.monitorCenter.upstream.componentStatusHint') }}</div>
        </div>
      </div>
      <div class="mc-upstream-meta">
        <span>{{ t('admin.monitorCenter.upstream.lastSync', { time: formatDateTime(status?.last_success_at) }) }}</span>
        <span>{{ t('admin.monitorCenter.upstream.rangeIncidentCount', { range: rangeLabel, count: incidents.length }) }}</span>
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
          <span :style="{ color: STATUS_COLORS[group.status] }">{{ statusLabel(t, group.status) }}</span>
        </div>
        <div class="mc-group-availability">
          <span>{{ t('admin.monitorCenter.upstream.rangeAvailability', { range: rangeLabel }) }}</span>
          <strong>{{ formatPercent(history?.statistics?.groups?.[group.key]?.availability_pct, 2) }}</strong>
        </div>
        <div class="mc-component-list">
          <div v-for="component in group.components" :key="component.key">
            <span>{{ component.name }}</span>
            <span :style="{ color: STATUS_COLORS[component.status] }">{{ component.matched ? statusLabel(t, component.status) : t('admin.monitorCenter.upstream.notReported') }}</span>
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
        <div v-if="historySlots(group).length" class="mc-band">
          <i
            v-for="(item, index) in historySlots(group)"
            :key="index"
            :class="[{ missing: item.status == null, linked: item.incidentIDs.length > 0, 'fetch-failed': item.fetchFailed }]"
            :style="{ backgroundColor: STATUS_COLORS[item.status ?? 'unknown'] }"
            :title="slotTitle(item)"
          />
        </div>
        <div v-else class="mc-band-empty">{{ t('common.noData') }}</div>
      </div>
      <div class="mc-band-legend">
        <span v-for="status in (['operational', 'degraded_performance', 'partial_outage', 'major_outage', 'under_maintenance', 'unknown'] as MonitorCenterStatus[])" :key="status"><i :style="{ backgroundColor: STATUS_COLORS[status] }" />{{ statusLabel(t, status) }}</span>
        <span><i class="fetch-failed" />{{ t('admin.monitorCenter.history.fetchFailed') }}</span>
        <span><i class="linked" />{{ t('admin.monitorCenter.upstream.relatedRisk') }}</span>
      </div>
    </div>

    <section class="mc-incidents">
      <div class="mc-incidents-head">
        <div>
          <strong>{{ t('admin.monitorCenter.upstream.officialEvents') }}</strong>
          <span>{{ t('admin.monitorCenter.upstream.officialEventsHint') }}</span>
        </div>
        <span class="mc-badge" :class="incidents.some(item => item.status !== 'resolved') ? 'warn' : 'good'">{{ incidents.length }}</span>
      </div>
      <div v-if="incidents.length" class="mc-incident-layout">
        <div class="mc-incident-list">
          <button v-for="incident in incidents" :key="incident.id" type="button" :class="{ active: selectedIncidentID === incident.id }" @click="selectedIncidentID = incident.id">
            <i :class="incidentTone(incident)" />
            <span><strong>{{ incident.name }}</strong><small>{{ formatDateTime(incidentStart(incident)) }} · {{ incidentDuration(incident) }}</small></span>
            <ChevronRight />
          </button>
        </div>
        <div v-if="selectedIncident" class="mc-incident-detail">
          <div class="mc-incident-title">
            <div><strong>{{ selectedIncident.name }}</strong><span>{{ selectedIncident.status }} · {{ selectedIncident.impact }}</span></div>
            <a :href="incidentURL(selectedIncident)" target="_blank" rel="noopener noreferrer"><ExternalLink />{{ t('admin.monitorCenter.upstream.officialDetails') }}</a>
          </div>
          <dl>
            <div><dt>{{ t('admin.monitorCenter.upstream.startedAt') }}</dt><dd>{{ formatDateTime(incidentStart(selectedIncident)) }}</dd></div>
            <div><dt>{{ t('admin.monitorCenter.upstream.updatedAt') }}</dt><dd>{{ formatDateTime(selectedIncident.updated_at) }}</dd></div>
            <div><dt>{{ t('admin.monitorCenter.upstream.duration') }}</dt><dd>{{ incidentDuration(selectedIncident) }}</dd></div>
            <div><dt>{{ t('admin.monitorCenter.upstream.affected') }}</dt><dd>{{ selectedIncident.affected_components.join(' · ') || '-' }}</dd></div>
          </dl>
          <div class="mc-incident-timeline">
            <div v-for="update in selectedIncident.updates" :key="`${update.updated_at}-${update.status}`">
              <i />
              <span><time>{{ formatDateTime(update.updated_at) }}</time><strong>{{ update.status }}</strong><p>{{ update.body }}</p></span>
            </div>
          </div>
        </div>
      </div>
      <div v-else class="mc-incident-empty"><CheckCircle2 /><span><strong>{{ t('admin.monitorCenter.upstream.noIncidentsInRange') }}</strong><small>{{ t('admin.monitorCenter.upstream.noIncidentsHint') }}</small></span></div>
    </section>
  </article>
</template>

<style scoped>
.mc-upstream { min-width: 0; }
.mc-upstream-head,.mc-incidents-head,.mc-incident-title { display:flex; align-items:flex-start; justify-content:space-between; gap:16px; }
.mc-upstream-copy { min-width:0; }
.mc-upstream-meta { display:grid; gap:4px; color:var(--mc-subtle); font-size:9px; text-align:right; }
.mc-stale { display:flex; align-items:center; gap:7px; margin-top:10px; }
.mc-stale svg { width:14px; height:14px; flex:none; }
.mc-groups { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:7px; margin-top:11px; }
.mc-group { min-width:0; border:1px solid color-mix(in srgb,var(--mc-line) 72%,transparent); border-radius:7px; padding:10px; background:var(--mc-soft); }
.mc-group-head,.mc-group-availability { display:flex; align-items:center; justify-content:space-between; gap:8px; }
.mc-group-head { font-size:11px; }
.mc-group-head>span { font-size:9px; font-weight:700; }
.mc-group-availability { margin-top:7px; border-bottom:1px solid var(--mc-line); padding-bottom:7px; color:var(--mc-subtle); font-size:8px; }
.mc-group-availability strong { color:var(--mc-text); font-size:10px; font-variant-numeric:tabular-nums; }
.mc-component-list { display:grid; gap:5px; margin-top:8px; }
.mc-component-list>div { display:flex; justify-content:space-between; gap:8px; color:var(--mc-muted); font-size:9px; }
.mc-component-list>div span:first-child { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.mc-component-list>div span:last-child { flex:none; font-weight:650; }
.mc-upstream-empty { min-height:94px; }
.mc-bands { display:grid; gap:6px; margin-top:11px; border-top:1px solid var(--mc-line); padding-top:10px; }
.mc-band-summary { display:flex; justify-content:space-between; gap:10px; color:var(--mc-subtle); font-size:9px; }
.mc-band-summary span:first-child { color:var(--mc-muted); font-weight:650; }
.mc-band-row { display:grid; grid-template-columns:56px minmax(0,1fr); align-items:center; gap:8px; }
.mc-band-row>span { color:var(--mc-muted); font-size:9px; font-weight:650; }
.mc-band { display:grid; grid-auto-flow:column; grid-auto-columns:minmax(3px,1fr); gap:2px; height:11px; }
.mc-band i,.mc-band-legend i { display:block; min-width:0; border-radius:2px; background:var(--mc-subtle); }
.mc-band i.good,.mc-band-legend i.good { background:var(--mc-green); }
.mc-band i.warn,.mc-band-legend i.warn { background:var(--mc-orange); }
.mc-band i.bad,.mc-band-legend i.bad { background:var(--mc-red); }
.mc-band i.missing,.mc-band-legend i.missing { background:color-mix(in srgb,var(--mc-subtle) 35%,transparent); }
.mc-band i.linked { box-shadow:inset 0 -2px var(--mc-blue); }
.mc-band i.fetch-failed { outline:1px solid var(--mc-subtle); outline-offset:-2px; }
.mc-band-legend { display:flex; flex-wrap:wrap; justify-content:flex-end; gap:8px 12px; color:var(--mc-subtle); font-size:8px; }
.mc-band-legend span { display:inline-flex; align-items:center; gap:4px; }
.mc-band-legend i { width:8px; height:8px; }
.mc-band-legend i.linked { background:var(--mc-blue); }
.mc-band-legend i.fetch-failed { background:linear-gradient(to bottom,var(--mc-panel) 0 55%,var(--mc-subtle) 55%); }
.mc-band-empty { color:var(--mc-subtle); font-size:9px; }
.mc-incidents { margin-top:12px; border-top:1px solid var(--mc-line); padding-top:11px; }
.mc-incidents-head strong { display:block; font-size:10px; }
.mc-incidents-head span:not(.mc-badge) { display:block; margin-top:2px; color:var(--mc-subtle); font-size:9px; }
.mc-incident-layout { display:grid; grid-template-columns:minmax(190px,.7fr) minmax(0,1.3fr); gap:9px; margin-top:9px; }
.mc-incident-list { display:grid; align-content:start; gap:4px; max-height:238px; overflow:auto; }
.mc-incident-list button { display:grid; grid-template-columns:7px minmax(0,1fr) 12px; align-items:center; gap:7px; border:1px solid transparent; border-radius:6px; padding:8px; color:var(--mc-text); text-align:left; background:var(--mc-soft); }
.mc-incident-list button.active { border-color:color-mix(in srgb,var(--mc-blue) 45%,var(--mc-line)); background:color-mix(in srgb,var(--mc-blue) 6%,var(--mc-soft)); }
.mc-incident-list button>i { width:7px; height:7px; border-radius:50%; background:var(--mc-green); }
.mc-incident-list button>i.warn { background:var(--mc-orange); }
.mc-incident-list button>i.bad { background:var(--mc-red); }
.mc-incident-list button span { min-width:0; }
.mc-incident-list strong,.mc-incident-list small { display:block; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.mc-incident-list strong { font-size:9px; }
.mc-incident-list small { margin-top:3px; color:var(--mc-subtle); font-size:8px; }
.mc-incident-list svg { width:12px; height:12px; color:var(--mc-subtle); }
.mc-incident-detail { min-width:0; border-radius:7px; padding:10px; background:var(--mc-soft); }
.mc-incident-title strong,.mc-incident-title span { display:block; }
.mc-incident-title strong { font-size:10px; }
.mc-incident-title span { margin-top:3px; color:var(--mc-subtle); font-size:8px; }
.mc-incident-title a { display:inline-flex; align-items:center; gap:4px; color:var(--mc-blue); font-size:8px; white-space:nowrap; }
.mc-incident-title svg { width:11px; height:11px; }
.mc-incident-detail dl { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:7px; margin:10px 0; }
.mc-incident-detail dl>div { min-width:0; }
.mc-incident-detail dt { color:var(--mc-subtle); font-size:8px; }
.mc-incident-detail dd { overflow:hidden; margin:2px 0 0; color:var(--mc-muted); font-size:8px; text-overflow:ellipsis; white-space:nowrap; }
.mc-incident-timeline { display:grid; gap:0; max-height:125px; overflow:auto; border-top:1px solid var(--mc-line); padding-top:8px; }
.mc-incident-timeline>div { display:grid; grid-template-columns:8px minmax(0,1fr); gap:7px; }
.mc-incident-timeline i { position:relative; width:6px; height:6px; margin-top:3px; border-radius:50%; background:var(--mc-blue); }
.mc-incident-timeline i::after { position:absolute; top:7px; bottom:-100px; left:2px; width:1px; background:var(--mc-line); content:''; }
.mc-incident-timeline span { padding-bottom:8px; }
.mc-incident-timeline time { color:var(--mc-subtle); font-size:8px; }
.mc-incident-timeline strong { margin-left:6px; font-size:8px; }
.mc-incident-timeline p { margin:3px 0 0; color:var(--mc-muted); font-size:8px; line-height:1.45; }
.mc-incident-empty { display:flex; align-items:center; gap:8px; margin-top:9px; border-radius:7px; padding:10px; background:var(--mc-soft); }
.mc-incident-empty svg { width:14px; color:var(--mc-green); }
.mc-incident-empty strong,.mc-incident-empty small { display:block; font-size:9px; }
.mc-incident-empty small { margin-top:2px; color:var(--mc-subtle); font-size:8px; }
@media(max-width:760px){.mc-upstream-head{flex-direction:column}.mc-upstream-meta{text-align:left}.mc-groups,.mc-incident-layout{grid-template-columns:1fr}.mc-incident-detail dl{grid-template-columns:1fr}}
</style>
