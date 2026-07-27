<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { CalendarClock } from '@lucide/vue'
import type {
  MonitorCenterOpenAIHistoryPoint,
  MonitorCenterStatus,
  MonitorCenterThreeDayData,
  OpsErrorTrendResponse,
  OpsThroughputTrendResponse,
} from '@/api/admin/monitorCenter'
import { formatAxisTime, formatDateTime, formatMs, formatPercent, statusLabel, statusTone } from './monitorCenterUtils'

const props = defineProps<{
  data: MonitorCenterThreeDayData | null
  loading: boolean
}>()
const { t } = useI18n()

const openAIPoints = computed(() => props.data?.openai?.points ?? [])
const sampleCount = computed(() => openAIPoints.value.length)
const successfulSamples = computed(() => openAIPoints.value.filter((point) => point.fetch_status === 'success'))
const successRate = computed(() => sampleCount.value ? successfulSamples.value.length / sampleCount.value * 100 : null)
const averageLatency = computed(() => successfulSamples.value.length
  ? successfulSamples.value.reduce((total, point) => total + point.latency_ms, 0) / successfulSamples.value.length
  : null)
const anomalyCount = computed(() => openAIPoints.value.filter((point) => point.fetch_status !== 'success' || point.overall_status !== 'operational').length)
const overallTone = computed(() => anomalyCount.value > 0 ? 'warn' : sampleCount.value ? 'good' : 'unknown')

function sample<T>(points: T[], target = 72): T[] {
  if (!points.length) return []
  const step = Math.max(1, Math.ceil(points.length / target))
  return points.filter((_, index) => index % step === 0).slice(-target)
}

function openAIStatuses(key: 'api_status' | 'chatgpt_status' | 'codex_status'): MonitorCenterStatus[] {
  return sample(openAIPoints.value).map((point) => point.fetch_status === 'success' ? point[key] : 'unknown')
}

function gatewayStatusFor(requestCount: number, errorCount: number): MonitorCenterStatus {
  if (requestCount <= 0) return errorCount > 0 ? 'major_outage' : 'unknown'
  const rate = errorCount / requestCount * 100
  if (rate >= 5) return 'major_outage'
  if (rate >= 2) return 'partial_outage'
  if (rate >= 1) return 'degraded_performance'
  return 'operational'
}

function gatewayStatuses(throughput: OpsThroughputTrendResponse | undefined, errors: OpsErrorTrendResponse | undefined): MonitorCenterStatus[] {
  const errorsByTime = new Map((errors?.points ?? []).map((point) => [point.bucket_start, point]))
  return sample(throughput?.points ?? []).map((point) => gatewayStatusFor(
    point.request_count,
    errorsByTime.get(point.bucket_start)?.error_count_sla ?? 0,
  ))
}

const bands = computed(() => [
  { key: 'api', label: 'API', values: openAIStatuses('api_status') },
  { key: 'chatgpt', label: 'ChatGPT', values: openAIStatuses('chatgpt_status') },
  { key: 'codex', label: 'Codex', values: openAIStatuses('codex_status') },
  { key: 'gateway', label: t('admin.monitorCenter.history.gateway'), values: gatewayStatuses(props.data?.throughput, props.data?.errors) },
  { key: 'probe', label: t('admin.monitorCenter.history.probe'), values: sample(props.data?.probe?.points ?? []).map((point) => point.status) },
])

const recentOpenAI = computed(() => [...openAIPoints.value].reverse().slice(0, 4))

function overallPointStatus(point: MonitorCenterOpenAIHistoryPoint): MonitorCenterStatus {
  return point.fetch_status === 'success' ? point.overall_status : 'unknown'
}
</script>

<template>
  <section class="mc-panel mc-panel-pad">
    <div class="mc-panel-head mc-history-head">
      <div class="mc-status-row">
        <div class="mc-icon-tile"><CalendarClock /></div>
        <div>
          <div class="mc-panel-title">{{ t('admin.monitorCenter.history.title') }}</div>
          <div class="mc-panel-subtitle">{{ t('admin.monitorCenter.history.subtitle') }}</div>
        </div>
      </div>
      <span class="mc-badge" :class="overallTone">{{ t(`admin.monitorCenter.history.${overallTone}`) }}</span>
    </div>

    <div class="mc-history-layout">
      <div class="mc-history-bands">
        <div v-for="band in bands" :key="band.key" class="mc-history-row">
          <span>{{ band.label }}</span>
          <div
            v-if="band.values.length"
            class="mc-history-band"
            :style="{ gridTemplateColumns: `repeat(${band.values.length}, minmax(0, 1fr))` }"
          >
            <i v-for="(status, index) in band.values" :key="index" :class="statusTone(status)" :title="statusLabel(t, status)" />
          </div>
          <div v-else class="mc-history-empty">{{ loading ? t('common.loading') : t('common.noData') }}</div>
        </div>
        <div class="mc-history-axis">
          <span>{{ openAIPoints.length ? formatAxisTime(openAIPoints[0].timestamp, true) : t('admin.monitorCenter.history.threeDaysAgo') }}</span>
          <span>{{ t('admin.monitorCenter.history.now') }}</span>
        </div>
        <div class="mc-history-legend">
          <span v-for="status in (['operational', 'degraded_performance', 'partial_outage', 'major_outage', 'under_maintenance', 'unknown'] as MonitorCenterStatus[])" :key="status">
            <i :class="statusTone(status)" />{{ statusLabel(t, status) }}
          </span>
        </div>
      </div>

      <div class="mc-history-side">
        <div class="mc-history-kpis">
          <div><span>{{ t('admin.monitorCenter.history.samples') }}</span><strong>{{ sampleCount }}</strong></div>
          <div><span>{{ t('admin.monitorCenter.history.successRate') }}</span><strong>{{ formatPercent(successRate, 2) }}</strong></div>
          <div><span>{{ t('admin.monitorCenter.history.averageLatency') }}</span><strong>{{ formatMs(averageLatency) }}</strong></div>
          <div><span>{{ t('admin.monitorCenter.history.anomalies') }}</span><strong :class="anomalyCount ? 'mc-warn' : 'mc-good'">{{ anomalyCount }}</strong></div>
        </div>
        <div class="mc-history-list">
          <div v-for="point in recentOpenAI" :key="point.timestamp" class="mc-history-record">
            <time>{{ formatDateTime(point.timestamp) }}</time>
            <span>{{ t('admin.monitorCenter.history.officialSample') }} · {{ formatMs(point.latency_ms) }}</span>
            <strong :class="`mc-${statusTone(overallPointStatus(point))}`">{{ statusLabel(t, overallPointStatus(point)) }}</strong>
          </div>
          <div v-if="!recentOpenAI.length" class="mc-empty">{{ loading ? t('common.loading') : t('common.noData') }}</div>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.mc-history-head { align-items: center; }
.mc-history-layout { display: grid; grid-template-columns: minmax(0, 1.35fr) minmax(300px, .65fr); gap: 12px; }
.mc-history-bands { min-width: 0; border: 1px solid var(--mc-line); border-radius: 7px; padding: 15px 12px 11px; background: var(--mc-soft); }
.mc-history-row { display: grid; grid-template-columns: 62px minmax(0, 1fr); align-items: center; gap: 9px; margin-bottom: 12px; }
.mc-history-row > span { overflow: hidden; color: var(--mc-muted); font-size: 9px; font-weight: 650; text-overflow: ellipsis; white-space: nowrap; }
.mc-history-band { display: grid; min-width: 0; gap: 2px; height: 15px; overflow: hidden; }
.mc-history-band i { display: block; min-width: 0; border-radius: 2px; background: var(--mc-subtle); }
.mc-history-band i.good, .mc-history-legend i.good { background: var(--mc-green); }
.mc-history-band i.warn, .mc-history-legend i.warn { background: var(--mc-orange); }
.mc-history-band i.bad, .mc-history-legend i.bad { background: var(--mc-red); }
.mc-history-empty { color: var(--mc-subtle); font-size: 9px; }
.mc-history-axis { display: flex; justify-content: space-between; margin-left: 71px; color: var(--mc-subtle); font-size: 8px; }
.mc-history-legend { display: flex; flex-wrap: wrap; gap: 8px 12px; margin-top: 13px; border-top: 1px solid var(--mc-line); padding-top: 10px; color: var(--mc-muted); font-size: 8px; }
.mc-history-legend span { display: inline-flex; align-items: center; gap: 5px; }
.mc-history-legend i { width: 7px; height: 7px; border-radius: 2px; background: var(--mc-subtle); }
.mc-history-side { min-width: 0; }
.mc-history-kpis { display: grid; grid-template-columns: 1fr 1fr; gap: 7px; }
.mc-history-kpis > div { min-width: 0; border-radius: 7px; padding: 11px; background: var(--mc-soft); }
.mc-history-kpis span { display: block; color: var(--mc-subtle); font-size: 9px; font-weight: 650; text-transform: uppercase; }
.mc-history-kpis strong { display: block; overflow: hidden; margin-top: 5px; font-size: 17px; font-variant-numeric: tabular-nums; text-overflow: ellipsis; white-space: nowrap; }
.mc-history-list { display: grid; gap: 5px; margin-top: 8px; }
.mc-history-record { display: grid; grid-template-columns: 112px minmax(0, 1fr) auto; align-items: center; gap: 8px; border-radius: 6px; padding: 8px 9px; background: var(--mc-soft); font-size: 9px; }
.mc-history-record time { color: var(--mc-subtle); font-variant-numeric: tabular-nums; }
.mc-history-record span { overflow: hidden; color: var(--mc-muted); text-overflow: ellipsis; white-space: nowrap; }
.mc-history-record strong { font-size: 9px; }
.mc-history-list .mc-empty { min-height: 80px; }
@media (max-width: 1000px) { .mc-history-layout { grid-template-columns: 1fr; } }
@media (max-width: 600px) {
  .mc-history-record { grid-template-columns: 1fr auto; }
  .mc-history-record span { grid-column: 1 / -1; grid-row: 2; }
}
</style>
