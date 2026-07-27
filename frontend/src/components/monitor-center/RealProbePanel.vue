<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Route } from '@lucide/vue'
import { CategoryScale, Chart as ChartJS, LineElement, LinearScale, PointElement, Tooltip } from 'chart.js'
import { Line } from 'vue-chartjs'
import type { MonitorCenterProbeResponse } from '@/api/admin/monitorCenter'
import { chartPalette, formatAxisTime, formatDateTime, formatMs, statusLabel, statusTone } from './monitorCenterUtils'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip)

const props = defineProps<{
  probe: MonitorCenterProbeResponse | null
  loading: boolean
}>()

const { t } = useI18n()
const hasData = computed(() => (props.probe?.points?.length ?? 0) > 0)
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
</style>
