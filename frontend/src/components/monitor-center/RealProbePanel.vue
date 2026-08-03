<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Route } from '@lucide/vue'
import { CategoryScale, Chart as ChartJS, LineElement, LinearScale, PointElement, Tooltip } from 'chart.js'
import { Line } from 'vue-chartjs'
import type { MonitorCenterOpenAIHistoryResponse, MonitorCenterProbeResponse, MonitorCenterStatus } from '@/api/admin/monitorCenter'
import { chartPalette, formatAxisTime, formatDateTime, formatMs, statusLabel, statusTone } from './monitorCenterUtils'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip)

const props = defineProps<{
  probe: MonitorCenterProbeResponse | null
  officialHistory: MonitorCenterOpenAIHistoryResponse | null
  loading: boolean
}>()

const { t } = useI18n()
const hasData = computed(() => (props.probe?.points?.length ?? 0) > 0)
const currentProbePoint = computed(() => {
  if (!props.probe?.configured || !props.probe.last_checked_at) return null
  return {
    timestamp: props.probe.last_checked_at,
    status: props.probe.status,
    failure_reason: props.probe.failure_reason,
  }
})
const correspondingOfficialAPIStatus = computed<MonitorCenterStatus>(() => {
  if (!currentProbePoint.value) return 'unknown'
  const probeAt = new Date(currentProbePoint.value.timestamp).getTime()
  const closest = (props.officialHistory?.points ?? [])
    .filter(point => point.fetch_status === 'success' && Number.isFinite(new Date(point.timestamp).getTime()))
    .map(point => ({ point, distance: Math.abs(new Date(point.timestamp).getTime() - probeAt) }))
    .filter(item => item.distance <= 5 * 60_000)
    .sort((a, b) => a.distance - b.distance)[0]
  return closest?.point.api_status ?? 'unknown'
})
const relatedOfficialRisk = computed(() => (props.officialHistory?.incidents ?? []).some(incident => incident.affected_groups?.includes('api') && incident.status !== 'resolved'))
const attribution = computed(() => {
  const point = currentProbePoint.value
  if (!props.probe?.configured || !point) return { tone: 'unknown', key: 'insufficientEvidence' }
  const probeFailed = ['partial_outage', 'major_outage'].includes(point.status)
    || (point.status === 'unknown' && !!point.failure_reason)
  const officialFailed = !['operational', 'unknown'].includes(correspondingOfficialAPIStatus.value)
  if (probeFailed && officialFailed) return { tone: 'bad', key: 'suspectedUpstream' }
  if (probeFailed && correspondingOfficialAPIStatus.value === 'operational') return { tone: 'warn', key: 'localFirst' }
  if (probeFailed) return { tone: 'unknown', key: 'insufficientEvidence' }
  if (point.status === 'operational' && relatedOfficialRisk.value) return { tone: 'warn', key: 'availableWithRisk' }
  if (point.status === 'operational') return { tone: 'good', key: 'pathAvailable' }
  return { tone: 'unknown', key: 'insufficientEvidence' }
})
const chartData = computed(() => ({
  labels: (props.probe?.points ?? []).map((point) => formatAxisTime(point.timestamp)),
  datasets: [{
    label: t('admin.monitorCenter.probe.latency'),
    data: (props.probe?.points ?? []).map((point) => point.status === 'unknown' ? null : point.latency_ms ?? null),
    borderColor: '#4f9d73',
    backgroundColor: '#4f9d73',
    borderWidth: 2,
    pointRadius: 0,
    pointHitRadius: 8,
    tension: .3,
    spanGaps: false,
  }],
}))

const chartOptions = computed(() => {
  const palette = chartPalette()
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { intersect: false, mode: 'index' as const },
    plugins: {
      legend: { display: false },
      tooltip: {
        backgroundColor: palette.tooltipBg,
        borderColor: palette.tooltipBorder,
        borderWidth: 1,
        titleColor: palette.title,
        bodyColor: palette.text,
        callbacks: { label: (context: any) => `${t('admin.monitorCenter.probe.latency')}: ${formatMs(context.parsed.y)}` },
      },
    },
    scales: { x: { display: false }, y: { display: false, beginAtZero: true } },
  }
})
</script>

<template>
  <article class="mc-panel mc-panel-pad mc-compact-status" :class="{ 'mc-loading': loading && !probe }">
    <div class="mc-compact-head">
      <div class="mc-status-row">
        <div class="mc-icon-tile green"><Route /></div>
        <div>
          <div class="mc-panel-title">{{ t('admin.monitorCenter.probe.title') }}</div>
          <div class="mc-panel-subtitle">{{ probe?.configured ? (probe.endpoint_kind === 'openai_direct' ? t('admin.monitorCenter.probe.direct') : t('admin.monitorCenter.probe.customEndpoint')) : t('admin.monitorCenter.probe.notConfigured') }}</div>
        </div>
      </div>
      <span class="mc-badge" :class="statusTone(probe?.status)">{{ statusLabel(t, probe?.status) }}</span>
    </div>
    <div class="mc-status-value">
      <div><span>{{ t('admin.monitorCenter.probe.lastLatency') }}</span><strong>{{ formatMs(probe?.latency_ms) }}</strong></div>
      <span>{{ t('admin.monitorCenter.probe.failures', { count: probe?.consecutive_failures ?? 0 }) }}</span>
    </div>
    <div class="mc-probe-meta">
      <div><span>{{ t('admin.monitorCenter.probe.model') }}</span><strong :title="probe?.model">{{ probe?.model || '-' }}</strong></div>
      <div><span>{{ t('admin.monitorCenter.probe.lastSuccess') }}</span><strong>{{ formatDateTime(probe?.last_success_at) }}</strong></div>
    </div>
    <div class="mc-attribution" :class="attribution.tone">
      <strong>{{ t('admin.monitorCenter.probe.attribution') }}</strong>
      <span>{{ t(`admin.monitorCenter.probe.${attribution.key}`) }}</span>
    </div>
    <div class="mc-mini-chart">
      <Line v-if="hasData" :data="chartData" :options="chartOptions" />
      <div v-else class="mc-empty">{{ loading ? t('common.loading') : t('admin.monitorCenter.probe.noSamples') }}</div>
    </div>
  </article>
</template>

<style scoped>
.mc-compact-status { display: flex; min-height: 280px; flex-direction: column; }
.mc-compact-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 9px; }
.mc-icon-tile.green { color: var(--mc-green); background: color-mix(in srgb, var(--mc-green) 10%, transparent); }
.mc-status-value { display: flex; align-items: flex-end; justify-content: space-between; gap: 8px; margin-top: 15px; }
.mc-status-value div > span, .mc-probe-meta span { display: block; color: var(--mc-subtle); font-size: 9px; }
.mc-status-value strong { display: block; margin-top: 3px; font-size: 22px; font-weight: 720; font-variant-numeric: tabular-nums; }
.mc-status-value > span { color: var(--mc-muted); font-size: 9px; }
.mc-probe-meta { display: grid; grid-template-columns: 1fr 1.25fr; gap: 8px; margin-top: 12px; border-block: 1px solid var(--mc-line); padding-block: 9px; }
.mc-probe-meta > div { min-width: 0; }
.mc-probe-meta strong { display: block; overflow: hidden; margin-top: 3px; color: var(--mc-muted); font-size: 9px; font-weight: 620; text-overflow: ellipsis; white-space: nowrap; }
.mc-mini-chart { min-height: 0; height: 78px; margin-top: auto; padding-top: 9px; }
.mc-mini-chart .mc-empty { min-height: 70px; }
.mc-attribution { margin-top:8px; border-left:2px solid var(--mc-subtle); padding:6px 8px; background:var(--mc-soft); }
.mc-attribution.good { border-left-color:var(--mc-green); }
.mc-attribution.warn { border-left-color:var(--mc-orange); }
.mc-attribution.bad { border-left-color:var(--mc-red); }
.mc-attribution strong,.mc-attribution span { display:block; font-size:8px; }
.mc-attribution span { margin-top:2px; color:var(--mc-muted); line-height:1.4; }
</style>
