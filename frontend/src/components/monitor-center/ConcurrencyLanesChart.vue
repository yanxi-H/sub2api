<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RefreshCw, UsersRound } from '@lucide/vue'
import { CategoryScale, Chart as ChartJS, Legend, LineElement, LinearScale, PointElement, Tooltip } from 'chart.js'
import { Line } from 'vue-chartjs'
import { summarizeNumbers, type NumberSummary } from '@/utils/numberSummary'
import type {
  ConcurrencyPeak,
  ConcurrencySnapshot,
  OpsPercentiles,
  OpsUserConcurrencyTrendResponse,
  UserConcurrencyTrendPoint,
} from '@/api/admin/ops'
import { chartPalette, formatAxisTime, formatMs, formatNumber } from './monitorCenterUtils'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Legend, Tooltip)

type LaneKey = 'normal' | 'heavy' | 'recovery'
type StatisticKey = keyof NumberSummary

const props = defineProps<{
  data: OpsUserConcurrencyTrendResponse | null
  loading: boolean
}>()
const emit = defineEmits<{ refresh: [] }>()
const { t } = useI18n()
const selectedUserId = ref('')

const lanes = computed(() => [
  { key: 'normal' as const, color: '#4f9d73', title: t('admin.monitorCenter.concurrency.normal') },
  { key: 'heavy' as const, color: '#c98524', title: t('admin.monitorCenter.concurrency.heavy') },
  { key: 'recovery' as const, color: '#7569b7', title: t('admin.monitorCenter.concurrency.recovery') },
])
const statistics: Array<{ key: StatisticKey; label: string }> = [
  { key: 'p95', label: 'P95' },
  { key: 'p90', label: 'P90' },
  { key: 'p50', label: 'P50' },
  { key: 'avg', label: 'Avg' },
  { key: 'max', label: 'Max' },
]
const latencyKeys: Record<StatisticKey, keyof OpsPercentiles> = {
  p95: 'p95_ms', p90: 'p90_ms', p50: 'p50_ms', avg: 'avg_ms', max: 'max_ms',
}

const userOptions = computed(() => Object.entries(props.data?.users ?? {})
  .map(([id, user]) => ({ id, label: user.username || user.user_email || `#${id}`, email: user.user_email }))
  .sort((a, b) => a.label.localeCompare(b.label)))

watch(userOptions, (options) => {
  if (selectedUserId.value && !options.some((option) => option.id === selectedUserId.value)) selectedUserId.value = ''
})

function systemPeak(point: UserConcurrencyTrendPoint, lane: LaneKey): ConcurrencyPeak | undefined {
  return point.system_lanes?.[lane] ?? (lane === 'normal' ? point.system : undefined)
}

function userPeak(point: UserConcurrencyTrendPoint, lane: LaneKey): ConcurrencyPeak | undefined {
  if (!selectedUserId.value) return undefined
  return point.user_lanes?.[selectedUserId.value]?.[lane] ?? (lane === 'normal' ? point.users?.[selectedUserId.value] : undefined)
}

function currentLane(lane: LaneKey): ConcurrencySnapshot {
  return props.data?.current_lanes?.[lane] ?? (lane === 'normal' && props.data?.current ? props.data.current : { in_use: 0, waiting: 0, demand: 0 })
}

function hasLaneData(lane: LaneKey): boolean {
  return (props.data?.points ?? []).some((point) => (systemPeak(point, lane)?.peak_demand ?? 0) > 0)
}

function laneStatistics(lane: LaneKey): NumberSummary {
  return summarizeNumbers((props.data?.points ?? []).map((point) => systemPeak(point, lane)?.peak_demand ?? 0))
}

function latencyValue(lane: LaneKey, key: StatisticKey): number | null {
  return props.data?.latency_lanes?.[lane]?.[latencyKeys[key]] ?? null
}

function userLabel(): string {
  const user = props.data?.users?.[selectedUserId.value]
  return user?.username || user?.user_email || `#${selectedUserId.value}`
}

function laneChartData(lane: LaneKey, color: string) {
  const points = props.data?.points ?? []
  const datasets: any[] = [
    {
      label: t('admin.monitorCenter.concurrency.systemActive'),
      data: points.map((point) => systemPeak(point, lane)?.peak_in_use ?? 0),
      borderColor: color, backgroundColor: color, borderWidth: 2.2, pointRadius: 0, pointHitRadius: 8,
      tension: .2, spanGaps: false, metric: 'system',
    },
    {
      label: t('admin.monitorCenter.concurrency.systemQueue'),
      data: points.map((point) => systemPeak(point, lane)?.peak_waiting ?? 0),
      borderColor: '#c84d49', backgroundColor: '#c84d49', borderWidth: 1.5, borderDash: [4, 3],
      pointRadius: 0, pointHitRadius: 8, tension: .2, spanGaps: false, metric: 'queue',
    },
  ]
  if (selectedUserId.value) {
    datasets.push({
      label: userLabel(),
      data: points.map((point) => userPeak(point, lane)?.peak_demand ?? 0),
      borderColor: '#3178c6', backgroundColor: '#3178c6', borderWidth: 1.8,
      pointRadius: 0, pointHitRadius: 8, tension: .2, spanGaps: false, metric: 'user',
    })
  }
  return { labels: points.map((point) => formatAxisTime(point.bucket_start)), datasets }
}

function laneChartOptions(lane: LaneKey) {
  const palette = chartPalette()
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { intersect: false, mode: 'index' as const },
    plugins: {
      legend: {
        position: 'top' as const, align: 'start' as const,
        labels: { color: palette.text, usePointStyle: true, boxWidth: 7, padding: 10, font: { size: 9 } },
      },
      tooltip: {
        backgroundColor: palette.tooltipBg, borderColor: palette.tooltipBorder, borderWidth: 1,
        titleColor: palette.title, bodyColor: palette.text, padding: 9,
        callbacks: {
          label: (context: any) => {
            if (context.dataset.metric !== 'user') return `${context.dataset.label}: ${context.parsed.y}`
            const point = props.data?.points?.[context.dataIndex]
            const peak = point ? userPeak(point, lane) : undefined
            return `${userLabel()}: ${t('admin.monitorCenter.concurrency.tooltip', { demand: peak?.peak_demand ?? 0, active: peak?.peak_in_use ?? 0, waiting: peak?.peak_waiting ?? 0 })}`
          },
        },
      },
    },
    scales: {
      x: { grid: { display: false }, ticks: { color: palette.text, maxTicksLimit: 5, font: { size: 9 } } },
      y: { beginAtZero: true, grid: { color: palette.grid }, ticks: { color: palette.text, precision: 0, font: { size: 9 } } },
    },
  }
}
</script>

<template>
  <section class="mc-panel mc-panel-pad">
    <div class="mc-panel-head mc-concurrency-head">
      <div class="mc-status-row">
        <div class="mc-icon-tile"><UsersRound /></div>
        <div>
          <div class="mc-panel-title">{{ t('admin.monitorCenter.concurrency.title') }}</div>
          <div class="mc-panel-subtitle">{{ t('admin.monitorCenter.concurrency.subtitle') }}</div>
        </div>
      </div>
      <div class="mc-concurrency-actions">
        <select v-model="selectedUserId" class="mc-select" :disabled="!userOptions.length" :aria-label="t('admin.monitorCenter.concurrency.selectUser')">
          <option value="">{{ userOptions.length ? t('admin.monitorCenter.concurrency.selectUser') : t('admin.monitorCenter.concurrency.noUsers') }}</option>
          <option v-for="option in userOptions" :key="option.id" :value="option.id">{{ option.label }}{{ option.email && option.email !== option.label ? ` · ${option.email}` : '' }}</option>
        </select>
        <button type="button" class="mc-button mc-icon-button" :disabled="loading" :title="t('common.refresh')" @click="emit('refresh')">
          <RefreshCw :class="{ 'animate-spin': loading }" />
        </button>
      </div>
    </div>

    <div v-if="data && !data.coverage_complete" class="mc-notice mc-coverage">
      {{ t('admin.monitorCenter.concurrency.partialCoverage') }}
    </div>

    <div class="mc-lanes">
      <article v-for="lane in lanes" :key="lane.key" class="mc-lane" :data-lane="lane.key">
        <div class="mc-lane-head">
          <strong>{{ lane.title }}</strong>
          <span>{{ t('admin.monitorCenter.concurrency.current', { active: currentLane(lane.key).in_use, waiting: currentLane(lane.key).waiting }) }}</span>
        </div>
        <div class="mc-stat-block">
          <div>
            <span>{{ t('admin.monitorCenter.concurrency.demand') }}</span>
            <div class="mc-five-stats">
              <div v-for="stat in statistics" :key="stat.key"><label>{{ stat.label }}</label><strong>{{ formatNumber(laneStatistics(lane.key)[stat.key], 2) }}</strong></div>
            </div>
          </div>
          <div>
            <span>{{ t('admin.monitorCenter.concurrency.responseTime') }}</span>
            <div class="mc-five-stats">
              <div v-for="stat in statistics" :key="stat.key"><label>{{ stat.label }}</label><strong :title="formatMs(latencyValue(lane.key, stat.key))">{{ formatMs(latencyValue(lane.key, stat.key)) }}</strong></div>
            </div>
          </div>
        </div>
        <div class="mc-lane-chart">
          <Line v-if="hasLaneData(lane.key)" :data="laneChartData(lane.key, lane.color)" :options="laneChartOptions(lane.key)" />
          <div v-else class="mc-empty">{{ loading ? t('common.loading') : t('admin.monitorCenter.concurrency.laneEmpty') }}</div>
        </div>
      </article>
    </div>
  </section>
</template>

<style scoped>
.mc-concurrency-head { align-items: center; }
.mc-concurrency-actions { display: flex; align-items: center; gap: 7px; }
.mc-concurrency-actions .mc-select { width: min(260px, 40vw); }
.mc-coverage { margin-bottom: 10px; }
.mc-lanes { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); border-top: 1px solid var(--mc-line); }
.mc-lane { min-width: 0; padding: 14px; }
.mc-lane + .mc-lane { border-left: 1px solid var(--mc-line); }
.mc-lane-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.mc-lane-head strong { font-size: 12px; }
.mc-lane-head span { color: var(--mc-muted); font-size: 9px; }
.mc-stat-block { margin-top: 10px; border-block: 1px solid var(--mc-line); }
.mc-stat-block > div { padding: 8px 0; }
.mc-stat-block > div + div { border-top: 1px solid var(--mc-line); }
.mc-stat-block > div > span { display: block; margin-bottom: 6px; color: var(--mc-subtle); font-size: 9px; font-weight: 650; }
.mc-five-stats { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 3px; text-align: center; }
.mc-five-stats label { display: block; color: var(--mc-subtle); font-size: 8px; font-weight: 650; text-transform: uppercase; }
.mc-five-stats strong { display: block; overflow: hidden; margin-top: 2px; font-size: 9px; font-variant-numeric: tabular-nums; text-overflow: ellipsis; white-space: nowrap; }
.mc-lane-chart { height: 230px; margin-top: 7px; }
.mc-lane-chart .mc-empty { min-height: 230px; }
@media (max-width: 1180px) {
  .mc-lanes { grid-template-columns: 1fr; }
  .mc-lane + .mc-lane { border-top: 1px solid var(--mc-line); border-left: 0; }
}
@media (max-width: 760px) {
  .mc-concurrency-head { align-items: stretch; }
  .mc-concurrency-actions .mc-select { width: 100%; }
}
</style>
