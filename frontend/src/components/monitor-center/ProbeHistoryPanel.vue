<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { CalendarClock, CircleAlert } from '@lucide/vue'
import type {
  MonitorCenterStatus,
  MonitorCenterThreeDayData,
  OpsErrorTrendResponse,
  OpsThroughputTrendResponse,
} from '@/api/admin/monitorCenter'
import { STATUS_COLORS, STATUS_ORDER, formatAxisTime, formatDateTime, formatMs, formatPercent, statusLabel, statusTone } from './monitorCenterUtils'

const props = defineProps<{
  data: MonitorCenterThreeDayData | null
  rangeLabel: string
  loading: boolean
}>()
const { t } = useI18n()
const anomalyOnly = ref(false)
const selectedKey = ref('')

interface AuditCell {
  key: string
  timestamp: string
  endTimestamp: string
  status: MonitorCenterStatus
  latencyMs: number | null
  failureReason: string
  incidents: string[]
  samples: number
  fetchFailed: boolean
}

const openAIPoints = computed(() => props.data?.openai?.points ?? [])
const incidentNames = computed(() => new Map((props.data?.openai?.incidents ?? []).map(item => [item.id, item.name])))
const stats = computed(() => props.data?.openai?.statistics)
const sampleCount = computed(() => stats.value?.sample_count ?? openAIPoints.value.length)
const successRate = computed(() => stats.value?.fetch_success_pct ?? null)
const averageLatency = computed(() => stats.value?.average_latency_ms ?? null)
const anomalyCount = computed(() => stats.value?.anomaly_count ?? 0)
const overallTone = computed(() => anomalyCount.value > 0 ? 'warn' : sampleCount.value ? 'good' : 'unknown')

function severity(status: MonitorCenterStatus): number {
  return STATUS_ORDER[status]
}

function aggregateCells<T>(points: T[], timestamp: (point: T) => string, map: (point: T) => Omit<AuditCell, 'key' | 'endTimestamp' | 'samples'>, target = 96): AuditCell[] {
  if (!points.length) return []
  const start = new Date(props.data?.openai?.start_time ?? timestamp(points[0])).getTime()
  const end = new Date(props.data?.openai?.end_time ?? timestamp(points[points.length - 1])).getTime()
  if (!Number.isFinite(start) || !Number.isFinite(end) || end <= start) return []
  const bucketCount = Math.min(target, Math.max(1, Math.ceil((end - start) / 60_000)))
  const width = (end - start) / bucketCount
  const cells = new Map<number, AuditCell>()
  for (const point of points) {
    const at = new Date(timestamp(point)).getTime()
    if (!Number.isFinite(at) || at < start || at > end) continue
    const index = Math.min(bucketCount - 1, Math.floor((at - start) / width))
    const next = map(point)
    const existing = cells.get(index)
    if (!existing) {
      cells.set(index, {
        ...next,
        key: `${index}-${next.timestamp}`,
        timestamp: new Date(start + index * width).toISOString(),
        endTimestamp: new Date(start + (index + 1) * width).toISOString(),
        samples: 1,
      })
    } else {
      existing.samples += 1
      existing.fetchFailed ||= next.fetchFailed
      if (!next.fetchFailed && severity(next.status) > severity(existing.status)) existing.status = next.status
      if (next.latencyMs != null && (existing.latencyMs == null || next.latencyMs > existing.latencyMs)) existing.latencyMs = next.latencyMs
      existing.failureReason ||= next.failureReason
      existing.incidents = [...new Set([...existing.incidents, ...next.incidents])]
    }
  }
  return Array.from({ length: bucketCount }, (_, index) => cells.get(index) ?? {
    key: `missing-${index}`,
    timestamp: new Date(start + index * width).toISOString(),
    endTimestamp: new Date(start + (index + 1) * width).toISOString(),
    status: 'unknown' as MonitorCenterStatus,
    latencyMs: null,
    failureReason: t('admin.monitorCenter.history.missingSampleReason'),
    incidents: [],
    samples: 0,
    fetchFailed: false,
  })
}

function officialCells(key: 'api_status' | 'chatgpt_status' | 'codex_status', group: string): AuditCell[] {
  return aggregateCells(openAIPoints.value, point => point.timestamp, point => ({
    timestamp: point.timestamp,
    status: point.fetch_status === 'success' ? point[key] : 'unknown',
    latencyMs: point.latency_ms,
    failureReason: point.failure_reason ?? '',
    incidents: point.incident_refs?.[group] ?? [],
    fetchFailed: point.fetch_status !== 'success',
  }))
}

function gatewayStatusFor(requestCount: number, errorCount: number): MonitorCenterStatus {
  if (requestCount <= 0) return errorCount > 0 ? 'major_outage' : 'unknown'
  const rate = errorCount / requestCount * 100
  if (rate >= 5) return 'major_outage'
  if (rate >= 2) return 'partial_outage'
  if (rate >= 1) return 'degraded_performance'
  return 'operational'
}

function gatewayCells(throughput: OpsThroughputTrendResponse | undefined, errors: OpsErrorTrendResponse | undefined): AuditCell[] {
  const errorsByTime = new Map((errors?.points ?? []).map(point => [point.bucket_start, point]))
  return aggregateCells(throughput?.points ?? [], point => point.bucket_start, point => {
    const errorPoint = errorsByTime.get(point.bucket_start)
    return {
      timestamp: point.bucket_start,
      status: gatewayStatusFor(point.request_count, errorPoint?.error_count_sla ?? 0),
      latencyMs: null,
      failureReason: point.request_count ? '' : t('admin.monitorCenter.history.noGatewayTraffic'),
      incidents: [],
      fetchFailed: false,
    }
  })
}

const bands = computed(() => [
  { key: 'api', label: 'API', values: officialCells('api_status', 'api') },
  { key: 'chatgpt', label: 'ChatGPT', values: officialCells('chatgpt_status', 'chatgpt') },
  { key: 'codex', label: 'Codex', values: officialCells('codex_status', 'codex') },
  { key: 'gateway', label: t('admin.monitorCenter.history.gateway'), values: gatewayCells(props.data?.throughput, props.data?.errors) },
  { key: 'probe', label: t('admin.monitorCenter.history.probe'), values: aggregateCells(props.data?.probe?.points ?? [], point => point.timestamp, point => ({
    timestamp: point.timestamp,
    status: point.status,
    latencyMs: point.latency_ms ?? null,
    failureReason: point.failure_reason ?? '',
    incidents: [],
    fetchFailed: false,
  })) },
])

const selectedCell = computed(() => bands.value.flatMap(band => band.values.map(cell => ({ ...cell, band: band.label }))).find(cell => `${cell.band}-${cell.key}` === selectedKey.value) ?? null)

const records = computed(() => bands.value
  .flatMap(band => band.values
    .filter(cell => cell.samples > 0)
    .map(cell => ({ ...cell, band: band.label, selectionKey: `${band.label}-${cell.key}` })))
  .filter(cell => !anomalyOnly.value || cell.fetchFailed || cell.status !== 'operational')
  .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime()))
function incidentSummary(ids: string[]): string {
  return ids.map(id => incidentNames.value.get(id) ?? id).join(' · ')
}
function cellTitle(cell: AuditCell): string {
  return `${formatDateTime(cell.timestamp)} - ${formatDateTime(cell.endTimestamp)} · ${statusLabel(t, cell.status)} · ${t('admin.monitorCenter.history.sampleCount', { count: cell.samples })}${cell.failureReason ? ` · ${cell.failureReason}` : ''}`
}
const axisTicks = computed(() => {
  const start = new Date(props.data?.openai?.start_time ?? '').getTime()
  const end = new Date(props.data?.openai?.end_time ?? '').getTime()
  if (!Number.isFinite(start) || !Number.isFinite(end) || end <= start) return []
  return Array.from({ length: 5 }, (_, i) => formatAxisTime(new Date(start + (end - start) * i / 4).toISOString(), true))
})
watch(bands, value => {
  const selectionStillExists = value.some(band => band.values.some(cell => `${band.label}-${cell.key}` === selectedKey.value))
  if (selectionStillExists) return
  const latest = [...(value[0]?.values ?? [])].reverse().find(cell => cell.samples > 0)
  selectedKey.value = latest ? `${value[0].label}-${latest.key}` : ''
}, { immediate: true })
</script>

<template>
  <section class="mc-panel mc-panel-pad">
    <div class="mc-panel-head mc-history-head">
      <div class="mc-status-row">
        <div class="mc-icon-tile"><CalendarClock /></div>
        <div><div class="mc-panel-title">{{ t('admin.monitorCenter.history.title') }}</div><div class="mc-panel-subtitle">{{ t('admin.monitorCenter.history.rangeSubtitle', { range: rangeLabel }) }}</div></div>
      </div>
      <span class="mc-badge" :class="overallTone">{{ t(`admin.monitorCenter.history.${overallTone}`) }}</span>
    </div>

    <div class="mc-history-kpis">
      <div><span>{{ t('admin.monitorCenter.history.samples') }}</span><strong>{{ sampleCount }}</strong></div>
      <div><span>{{ t('admin.monitorCenter.history.successRate') }}</span><strong>{{ formatPercent(successRate, 2) }}</strong></div>
      <div><span>{{ t('admin.monitorCenter.history.averageLatency') }}</span><strong>{{ formatMs(averageLatency) }}</strong></div>
      <div><span>{{ t('admin.monitorCenter.history.anomalies') }}</span><strong :class="anomalyCount ? 'mc-warn' : 'mc-good'">{{ anomalyCount }}</strong></div>
    </div>

    <div class="mc-history-layout">
      <div class="mc-history-main">
        <div class="mc-history-bands">
          <div v-for="band in bands" :key="band.key" class="mc-history-row">
            <span>{{ band.label }}</span>
            <div v-if="band.values.length" class="mc-history-band" :style="{ gridTemplateColumns: `repeat(${band.values.length}, minmax(2px, 1fr))` }">
              <button v-for="cell in band.values" :key="cell.key" type="button" :class="[{ empty: cell.samples === 0, selected: selectedKey === `${band.label}-${cell.key}`, 'fetch-failed': cell.fetchFailed }]" :style="{ backgroundColor: STATUS_COLORS[cell.status] }" :title="cellTitle(cell)" @click="selectedKey = `${band.label}-${cell.key}`" />
            </div>
            <div v-else class="mc-history-empty">{{ loading ? t('common.loading') : t('common.noData') }}</div>
          </div>
          <div class="mc-history-axis"><span v-for="tick in axisTicks" :key="tick">{{ tick }}</span></div>
          <div class="mc-history-legend">
            <span v-for="status in (['operational', 'degraded_performance', 'partial_outage', 'major_outage', 'under_maintenance', 'unknown'] as MonitorCenterStatus[])" :key="status"><i :style="{ backgroundColor: STATUS_COLORS[status] }" />{{ statusLabel(t, status) }}</span>
            <span><i class="fetch-failed" />{{ t('admin.monitorCenter.history.fetchFailed') }}</span>
          </div>
        </div>
        <div v-if="selectedCell" class="mc-sample-detail">
          <div><strong>{{ selectedCell.band }}</strong><span>{{ formatDateTime(selectedCell.timestamp) }} - {{ formatDateTime(selectedCell.endTimestamp) }}</span></div>
          <dl>
            <div><dt>{{ t('admin.monitorCenter.history.status') }}</dt><dd :class="`mc-${statusTone(selectedCell.status)}`">{{ statusLabel(t, selectedCell.status) }}</dd></div>
            <div><dt>{{ t('admin.monitorCenter.history.samples') }}</dt><dd>{{ selectedCell.samples }}</dd></div>
            <div><dt>{{ t('admin.monitorCenter.history.latency') }}</dt><dd>{{ formatMs(selectedCell.latencyMs) }}</dd></div>
            <div><dt>{{ t('admin.monitorCenter.history.relatedIncident') }}</dt><dd>{{ incidentSummary(selectedCell.incidents) || '-' }}</dd></div>
          </dl>
          <p v-if="selectedCell.failureReason"><CircleAlert />{{ selectedCell.failureReason }}</p>
        </div>
      </div>

      <aside class="mc-history-side">
        <div class="mc-history-list-head"><strong>{{ t('admin.monitorCenter.history.records') }}</strong><label><input v-model="anomalyOnly" type="checkbox" />{{ t('admin.monitorCenter.history.anomalyOnly') }}</label></div>
        <div class="mc-history-list">
          <button v-for="point in records" :key="point.selectionKey" type="button" class="mc-history-record" @click="selectedKey = point.selectionKey">
            <time>{{ formatDateTime(point.timestamp) }}</time>
            <span>{{ point.failureReason || `${point.band} · ${formatMs(point.latencyMs)}` }}</span>
            <strong :style="{ color: STATUS_COLORS[point.status] }">{{ statusLabel(t, point.status) }}</strong>
            <small v-if="point.incidents.length">{{ incidentSummary(point.incidents) }}</small>
          </button>
          <div v-if="!records.length" class="mc-empty">{{ loading ? t('common.loading') : t('common.noData') }}</div>
        </div>
      </aside>
    </div>
  </section>
</template>

<style scoped>
.mc-history-head{align-items:center}.mc-history-kpis{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:7px;margin-bottom:10px}.mc-history-kpis>div{min-width:0;border-radius:7px;padding:9px 11px;background:var(--mc-soft)}.mc-history-kpis span{display:block;color:var(--mc-subtle);font-size:8px;font-weight:650;text-transform:uppercase}.mc-history-kpis strong{display:block;margin-top:4px;font-size:15px;font-variant-numeric:tabular-nums}.mc-history-layout{display:grid;grid-template-columns:minmax(0,1.4fr) minmax(290px,.6fr);gap:12px}.mc-history-main,.mc-history-side{min-width:0}.mc-history-bands{border:1px solid var(--mc-line);border-radius:7px;padding:14px 11px 10px;background:var(--mc-soft)}.mc-history-row{display:grid;grid-template-columns:62px minmax(0,1fr);align-items:center;gap:9px;margin-bottom:10px}.mc-history-row>span{overflow:hidden;color:var(--mc-muted);font-size:9px;font-weight:650;text-overflow:ellipsis;white-space:nowrap}.mc-history-band{display:grid;min-width:0;gap:1px;height:17px}.mc-history-band button{min-width:0;border:0;border-radius:1px;padding:0;background:var(--mc-subtle);cursor:crosshair}.mc-history-band button.fetch-failed{box-shadow:inset 0 -3px var(--mc-subtle)}.mc-history-band button.empty{opacity:.28}.mc-history-band button.selected{outline:2px solid var(--mc-blue);outline-offset:1px}.mc-history-empty{color:var(--mc-subtle);font-size:9px}.mc-history-axis{display:flex;justify-content:space-between;margin-left:71px;color:var(--mc-subtle);font-size:8px}.mc-history-legend{display:flex;flex-wrap:wrap;gap:8px 12px;margin-top:11px;border-top:1px solid var(--mc-line);padding-top:9px;color:var(--mc-muted);font-size:8px}.mc-history-legend span{display:inline-flex;align-items:center;gap:5px}.mc-history-legend i{width:7px;height:7px;border-radius:2px;background:var(--mc-subtle)}.mc-history-legend i.fetch-failed{background:linear-gradient(to bottom,var(--mc-panel) 0 55%,var(--mc-subtle) 55%)}.mc-sample-detail{margin-top:8px;border-radius:7px;padding:10px;background:var(--mc-soft)}.mc-sample-detail>div{display:flex;justify-content:space-between;gap:8px;font-size:9px}.mc-sample-detail>div span{color:var(--mc-subtle)}.mc-sample-detail dl{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:8px;margin:9px 0 0}.mc-sample-detail dt{color:var(--mc-subtle);font-size:8px}.mc-sample-detail dd{overflow:hidden;margin:2px 0 0;color:var(--mc-muted);font-size:8px;text-overflow:ellipsis;white-space:nowrap}.mc-sample-detail p{display:flex;gap:5px;margin:8px 0 0;border-top:1px solid var(--mc-line);padding-top:7px;color:var(--mc-red);font-size:8px;line-height:1.4}.mc-sample-detail p svg{width:11px;height:11px;flex:none}.mc-history-list-head{display:flex;align-items:center;justify-content:space-between;gap:8px;margin-bottom:7px;font-size:9px}.mc-history-list-head label{display:flex;align-items:center;gap:4px;color:var(--mc-muted);font-size:8px}.mc-history-list{display:grid;align-content:start;gap:4px;max-height:330px;overflow:auto;padding-right:2px}.mc-history-record{display:grid;grid-template-columns:108px minmax(0,1fr) auto;align-items:center;gap:7px;border:0;border-radius:6px;padding:8px;color:var(--mc-text);text-align:left;background:var(--mc-soft);font-size:8px}.mc-history-record time{color:var(--mc-subtle);font-variant-numeric:tabular-nums}.mc-history-record span{overflow:hidden;color:var(--mc-muted);text-overflow:ellipsis;white-space:nowrap}.mc-history-record strong{font-size:8px}.mc-history-record small{grid-column:1/-1;overflow:hidden;color:var(--mc-blue);text-overflow:ellipsis;white-space:nowrap}.mc-history-list .mc-empty{min-height:80px}@media(max-width:1000px){.mc-history-layout{grid-template-columns:1fr}}@media(max-width:600px){.mc-history-kpis{grid-template-columns:1fr 1fr}.mc-sample-detail dl{grid-template-columns:1fr 1fr}.mc-history-record{grid-template-columns:1fr auto}.mc-history-record span{grid-column:1/-1;grid-row:2}}
</style>
