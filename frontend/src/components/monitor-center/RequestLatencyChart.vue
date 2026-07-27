<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Clock3 } from '@lucide/vue'
import { CategoryScale, Chart as ChartJS, Legend, LineElement, LinearScale, PointElement, Tooltip } from 'chart.js'
import { Line } from 'vue-chartjs'
import type { OpsDashboardOverview } from '@/api/admin/monitorCenter'
import type { OpsLatencyTrendPoint } from '@/api/admin/ops'
import { chartPalette, formatAxisTime, formatDateTime, formatMs, nullableMetric } from './monitorCenterUtils'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Legend, Tooltip)

type MetricMode = 'e2e' | 'ttft'
type PercentileKey = 'p95_ms' | 'p90_ms' | 'p50_ms' | 'avg_ms' | 'max_ms'

const props = defineProps<{
  points: OpsLatencyTrendPoint[]
  overview: OpsDashboardOverview | null
  loading: boolean
  range: string
}>()

const { t } = useI18n()
const mode = ref<MetricMode>('e2e')
const definitions: Array<{ key: PercentileKey; label: string; color: string; dash?: number[] }> = [
  { key: 'p95_ms', label: 'P95', color: '#c98524' },
  { key: 'p90_ms', label: 'P90', color: '#3178c6' },
  { key: 'p50_ms', label: 'P50', color: '#4f9d73' },
  { key: 'avg_ms', label: 'Avg', color: '#35889d' },
  { key: 'max_ms', label: 'Max', color: '#c84d49', dash: [5, 4] },
]

const hasData = computed(() => props.points.some((point) => point.sample_count > 0))
const labels = computed(() => props.points.map((point) => formatAxisTime(point.bucket_start, props.range === '24h' || props.range === 'custom')))

function pointValue(point: OpsLatencyTrendPoint, key: PercentileKey): number | null {
  return nullableMetric(mode.value === 'ttft' ? point.ttft?.[key] : point[key])
}

const chartData = computed(() => ({
  labels: labels.value,
  datasets: definitions.map((definition) => ({
    label: definition.label,
    data: props.points.map((point) => point.sample_count > 0 ? pointValue(point, definition.key) : null),
    borderColor: definition.color,
    backgroundColor: definition.color,
    borderWidth: definition.key === 'max_ms' ? 1.5 : 2,
    borderDash: definition.dash,
    pointRadius: 0,
    pointHitRadius: 10,
    tension: .25,
    spanGaps: false,
  })),
}))

const chartOptions = computed(() => {
  const palette = chartPalette()
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { intersect: false, mode: 'index' as const },
    plugins: {
      legend: {
        position: 'top' as const,
        align: 'end' as const,
        labels: { color: palette.text, usePointStyle: true, boxWidth: 7, padding: 14, font: { size: 10 } },
      },
      tooltip: {
        backgroundColor: palette.tooltipBg,
        borderColor: palette.tooltipBorder,
        borderWidth: 1,
        titleColor: palette.title,
        bodyColor: palette.text,
        padding: 10,
        callbacks: {
          title: (items: any[]) => {
            const index = items[0]?.dataIndex
            return typeof index === 'number' ? formatDateTime(props.points[index]?.bucket_start) : ''
          },
          label: (context: any) => `${context.dataset.label}: ${formatMs(context.parsed.y)}`,
        },
      },
    },
    scales: {
      x: {
        grid: { display: false },
        ticks: { color: palette.text, maxTicksLimit: 9, autoSkip: true, font: { size: 10 } },
      },
      y: {
        beginAtZero: true,
        grid: { color: palette.grid },
        ticks: { color: palette.text, callback: (value: string | number) => formatMs(Number(value)), font: { size: 10 } },
      },
    },
  }
})

const p99 = computed(() => mode.value === 'e2e' ? props.overview?.duration?.p99_ms : props.overview?.ttft?.p99_ms)
</script>

<template>
  <section class="mc-panel mc-panel-pad">
    <div class="mc-panel-head mc-latency-head">
      <div class="mc-status-row">
        <div class="mc-icon-tile"><Clock3 /></div>
        <div>
          <div class="mc-panel-title">{{ t('admin.monitorCenter.latency.title') }}</div>
          <div class="mc-panel-subtitle">{{ t('admin.monitorCenter.latency.subtitle') }}</div>
        </div>
      </div>
      <div class="mc-latency-actions">
        <div class="mc-p99"><span>P99</span><strong>{{ formatMs(p99) }}</strong></div>
        <div class="mc-segmented" role="tablist" :aria-label="t('admin.monitorCenter.latency.mode')">
          <button type="button" role="tab" :aria-selected="mode === 'e2e'" :tabindex="mode === 'e2e' ? 0 : -1" :class="{ active: mode === 'e2e' }" @click="mode = 'e2e'">E2E</button>
          <button type="button" role="tab" :aria-selected="mode === 'ttft'" :tabindex="mode === 'ttft' ? 0 : -1" :class="{ active: mode === 'ttft' }" @click="mode = 'ttft'">TTFT</button>
        </div>
      </div>
    </div>
    <div v-if="hasData" class="mc-chart mc-latency-chart">
      <Line :data="chartData" :options="chartOptions" />
    </div>
    <div v-else class="mc-empty mc-latency-chart">{{ loading ? t('common.loading') : t('admin.monitorCenter.latency.noData') }}</div>
  </section>
</template>

<style scoped>
.mc-latency-head { align-items: center; }
.mc-latency-actions { display: flex; align-items: center; gap: 14px; }
.mc-p99 { text-align: right; }
.mc-p99 span { display: block; color: var(--mc-subtle); font-size: 9px; font-weight: 700; }
.mc-p99 strong { display: block; margin-top: 2px; font-size: 13px; font-variant-numeric: tabular-nums; }
.mc-latency-chart { height: 330px; }
@media (max-width: 760px) {
  .mc-latency-head { align-items: stretch; }
  .mc-latency-actions { justify-content: space-between; }
}
</style>
