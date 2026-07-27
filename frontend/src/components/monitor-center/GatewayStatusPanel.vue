<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Router } from '@lucide/vue'
import { CategoryScale, Chart as ChartJS, Legend, LineElement, LinearScale, PointElement, Tooltip } from 'chart.js'
import { Line } from 'vue-chartjs'
import type {
  OpsDashboardOverview,
  OpsErrorTrendResponse,
  OpsThroughputTrendResponse,
} from '@/api/admin/monitorCenter'
import { chartPalette, formatAxisTime, formatNumber, formatPercent } from './monitorCenterUtils'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Legend, Tooltip)

const props = defineProps<{
  overview: OpsDashboardOverview | null
  errors: OpsErrorTrendResponse | null
  throughput: OpsThroughputTrendResponse | null
  loading: boolean
}>()

const { t } = useI18n()
const tone = computed(() => {
  const sla = props.overview?.sla
  const errorRate = props.overview?.error_rate
  if (sla == null || errorRate == null) return 'unknown'
  if (sla < 95 || errorRate >= 5) return 'bad'
  if (sla < 99 || errorRate >= 1) return 'warn'
  return 'good'
})

const chartData = computed(() => {
  const requestPoints = props.throughput?.points ?? []
  const errorsByTime = new Map((props.errors?.points ?? []).map((point) => [point.bucket_start, point]))
  return {
    labels: requestPoints.map((point) => formatAxisTime(point.bucket_start)),
    datasets: [
      {
        label: t('admin.monitorCenter.gateway.requests'),
        data: requestPoints.map((point) => point.request_count),
        borderColor: '#7569b7',
        backgroundColor: '#7569b7',
        yAxisID: 'y',
        borderWidth: 2,
        pointRadius: 0,
        tension: .3,
        spanGaps: false,
      },
      {
        label: t('admin.monitorCenter.gateway.errors'),
        data: requestPoints.map((point) => errorsByTime.get(point.bucket_start)?.error_count_total ?? null),
        borderColor: '#c84d49',
        backgroundColor: '#c84d49',
        yAxisID: 'y1',
        borderWidth: 1.5,
        borderDash: [4, 3],
        pointRadius: 0,
        tension: .3,
        spanGaps: false,
      },
    ],
  }
})

const hasData = computed(() => (props.throughput?.points?.length ?? 0) > 0)
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
      },
    },
    scales: {
      x: { display: false },
      y: { display: false, beginAtZero: true },
      y1: { display: false, beginAtZero: true },
    },
  }
})
</script>

<template>
  <article class="mc-panel mc-panel-pad mc-compact-status" :class="{ 'mc-loading': loading && !overview }">
    <div class="mc-compact-head">
      <div class="mc-status-row">
        <div class="mc-icon-tile purple"><Router /></div>
        <div>
          <div class="mc-panel-title">{{ t('admin.monitorCenter.gateway.title') }}</div>
          <div class="mc-panel-subtitle">{{ t('admin.monitorCenter.gateway.subtitle') }}</div>
        </div>
      </div>
      <span class="mc-badge" :class="tone">{{ t(`admin.monitorCenter.gateway.${tone}`) }}</span>
    </div>
    <div class="mc-status-value">
      <div><span>SLA</span><strong>{{ formatPercent(overview?.sla, 3) }}</strong></div>
      <span>{{ t('admin.monitorCenter.gateway.errorRate') }} {{ formatPercent(overview?.error_rate) }}</span>
    </div>
    <div class="mc-stat-pair">
      <div><span>{{ t('admin.monitorCenter.gateway.upstreamError') }}</span><strong>{{ formatPercent(overview?.upstream_error_rate) }}</strong></div>
      <div><span>{{ t('admin.monitorCenter.gateway.businessLimits') }}</span><strong>{{ formatNumber(overview?.business_limited_count, 0) }}</strong></div>
    </div>
    <div class="mc-mini-chart">
      <Line v-if="hasData" :data="chartData" :options="chartOptions" />
      <div v-else class="mc-empty">{{ loading ? t('common.loading') : t('common.noData') }}</div>
    </div>
  </article>
</template>

<style scoped>
.mc-compact-status { display: flex; min-height: 280px; flex-direction: column; }
.mc-compact-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 9px; }
.mc-icon-tile.purple { color: var(--mc-purple); background: color-mix(in srgb, var(--mc-purple) 10%, transparent); }
.mc-status-value { display: flex; align-items: flex-end; justify-content: space-between; gap: 8px; margin-top: 15px; }
.mc-status-value div > span, .mc-stat-pair span { display: block; color: var(--mc-subtle); font-size: 9px; }
.mc-status-value strong { display: block; margin-top: 3px; font-size: 22px; font-weight: 720; font-variant-numeric: tabular-nums; }
.mc-status-value > span { color: var(--mc-muted); font-size: 9px; }
.mc-stat-pair { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-top: 12px; border-block: 1px solid var(--mc-line); padding-block: 9px; }
.mc-stat-pair strong { display: block; margin-top: 3px; font-size: 12px; font-variant-numeric: tabular-nums; }
.mc-mini-chart { min-height: 0; height: 78px; margin-top: auto; padding-top: 9px; }
.mc-mini-chart .mc-empty { min-height: 70px; }
</style>
