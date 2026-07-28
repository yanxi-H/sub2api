<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Chart as ChartJS, CategoryScale, Legend, LineElement, LinearScale, PointElement, Tooltip } from 'chart.js'
import { Line } from 'vue-chartjs'
import { summarizeNumbers, type NumberSummary } from '@/utils/numberSummary'
import {
  opsAPI,
  type ConcurrencyPeak,
  type ConcurrencySnapshot,
  type OpsPercentiles,
  type OpsUserConcurrencyTrendResponse,
  type UserConcurrencyTrendPoint,
  type UserConcurrencyTrendUser
} from '@/api/admin/ops'

ChartJS.register(CategoryScale, Legend, LineElement, LinearScale, PointElement, Tooltip)

interface Props {
  refreshToken?: number
  timeRange: string
  customStartTime?: string | null
  customEndTime?: string | null
}

type LaneKey = 'normal' | 'heavy' | 'recovery'

interface LaneDefinition {
  key: LaneKey
  title: string
  color: string
  queueColor: string
}

const props = withDefaults(defineProps<Props>(), {
  refreshToken: 0,
  customStartTime: null,
  customEndTime: null
})
const { t } = useI18n()

const loading = ref(false)
const errorMessage = ref('')
const trend = ref<OpsUserConcurrencyTrendResponse | null>(null)
const selectedUserId = ref('')
const pinnedUserIds = ref<string[]>([])
let loadController: AbortController | null = null
let loadSequence = 0

const coverageWarning = computed(() => {
  const value = trend.value
  if (!value || value.coverage_complete) return ''
  if (value.coverage_start && value.coverage_end) {
    return t('admin.ops.concurrencyTrend.partialCoverage', {
      start: new Date(value.coverage_start).toLocaleString(),
      end: new Date(value.coverage_end).toLocaleString()
    })
  }
  return t('admin.ops.concurrencyTrend.unavailableCoverage')
})

const palette = ['#2563eb', '#059669', '#d97706', '#dc2626', '#7c3aed', '#0891b2', '#db2777', '#4f46e5', '#65a30d', '#ea580c']

const lanes = computed<LaneDefinition[]>(() => [
  { key: 'normal', title: t('admin.ops.concurrencyTrend.lanes.normal'), color: '#059669', queueColor: '#dc2626' },
  { key: 'heavy', title: t('admin.ops.concurrencyTrend.lanes.heavy'), color: '#d97706', queueColor: '#dc2626' },
  { key: 'recovery', title: t('admin.ops.concurrencyTrend.lanes.recovery'), color: '#7c3aed', queueColor: '#dc2626' }
])

const userOptions = computed(() => {
  const users = trend.value?.users || {}
  return Object.entries(users)
    .map(([id, user]) => ({ id, user, label: user.username || user.user_email || `#${id}` }))
    .sort((a, b) => a.label.localeCompare(b.label))
})

function pointUserLanePeak(point: UserConcurrencyTrendPoint, userId: string, lane: LaneKey): ConcurrencyPeak | undefined {
  const lanePeak = point.user_lanes?.[userId]?.[lane]
  if (lanePeak) return lanePeak
  return lane === 'normal' ? point.users?.[userId] : undefined
}

function pointSystemLanePeak(point: UserConcurrencyTrendPoint, lane: LaneKey): ConcurrencyPeak | undefined {
  const lanePeak = point.system_lanes?.[lane]
  if (lanePeak) return lanePeak
  return lane === 'normal' ? point.system : undefined
}

const rankedUserIdsByLane = computed<Record<LaneKey, string[]>>(() => {
  const peaksByLane: Record<LaneKey, Map<string, number>> = {
    normal: new Map<string, number>(),
    heavy: new Map<string, number>(),
    recovery: new Map<string, number>()
  }
  for (const point of trend.value?.points || []) {
    const userIds = new Set([...Object.keys(point.users || {}), ...Object.keys(point.user_lanes || {})])
    for (const userId of userIds) {
      for (const lane of lanes.value) {
        const demand = pointUserLanePeak(point, userId, lane.key)?.peak_demand || 0
        const peaks = peaksByLane[lane.key]
        peaks.set(userId, Math.max(peaks.get(userId) || 0, demand))
      }
    }
  }
  const ranked = (peaks: Map<string, number>) => [...peaks.entries()]
    .filter(([, peak]) => peak > 0)
    .sort((a, b) => b[1] - a[1])
    .map(([userId]) => userId)
  return {
    normal: ranked(peaksByLane.normal),
    heavy: ranked(peaksByLane.heavy),
    recovery: ranked(peaksByLane.recovery)
  }
})

function visibleUserIds(lane: LaneKey): string[] {
  const ids = [...rankedUserIdsByLane.value[lane].slice(0, 5)]
  for (const userId of pinnedUserIds.value) {
    if (!ids.includes(userId)) ids.push(userId)
  }
  return ids.slice(0, 10)
}

function userLabel(userId: string): string {
  const user = trend.value?.users?.[userId]
  return user?.username || user?.user_email || `#${userId}`
}

function userOptionText(user: UserConcurrencyTrendUser, fallback: string): string {
  const label = user.username || user.user_email || fallback
  return user.user_email && user.user_email !== label ? `${label} · ${user.user_email}` : label
}

function formatMinute(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false })
}

function currentLane(lane: LaneKey): ConcurrencySnapshot {
  const current = trend.value?.current_lanes?.[lane]
  if (current) return current
  if (lane === 'normal' && trend.value?.current) return trend.value.current
  return { in_use: 0, waiting: 0, demand: 0 }
}

function hasLaneData(lane: LaneKey): boolean {
  return (trend.value?.points || []).some(point => (pointSystemLanePeak(point, lane)?.peak_demand || 0) > 0)
}

function summarizeLane(lane: LaneKey): NumberSummary {
  return summarizeNumbers(
    (trend.value?.points || []).map(point => pointSystemLanePeak(point, lane)?.peak_demand || 0)
  )
}

const laneStatistics = computed<Record<LaneKey, NumberSummary>>(() => ({
  normal: summarizeLane('normal'),
  heavy: summarizeLane('heavy'),
  recovery: summarizeLane('recovery')
}))

const statisticDefinitions: Array<{ key: keyof NumberSummary; label: string }> = [
  { key: 'p95', label: 'P95' },
  { key: 'p90', label: 'P90' },
  { key: 'p50', label: 'P50' },
  { key: 'avg', label: 'Avg' },
  { key: 'max', label: 'Max' }
]

const latencyStatisticKeys: Record<keyof NumberSummary, keyof OpsPercentiles> = {
  p95: 'p95_ms',
  p90: 'p90_ms',
  p50: 'p50_ms',
  avg: 'avg_ms',
  max: 'max_ms'
}

function formatStatistic(value: number): string {
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(value)
}

function laneLatencyStatistic(lane: LaneKey, key: keyof NumberSummary): number | null {
  return trend.value?.latency_lanes?.[lane]?.[latencyStatisticKeys[key]] ?? null
}

function formatMilliseconds(value: number | null): string {
  if (value === null) return '-'
  return `${new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 }).format(value)} ms`
}

function laneChartData(lane: LaneDefinition) {
  const points = trend.value?.points || []
  const datasets: any[] = [
    {
      label: t('admin.ops.concurrencyTrend.systemActive'),
      data: points.map(point => pointSystemLanePeak(point, lane.key)?.peak_in_use || 0),
      borderColor: lane.color,
      backgroundColor: lane.color,
      borderWidth: 2.5,
      pointRadius: 1.5,
      pointHitRadius: 10,
      tension: 0.25,
      userId: null,
      metric: 'active'
    },
    {
      label: t('admin.ops.concurrencyTrend.systemWaiting'),
      data: points.map(point => pointSystemLanePeak(point, lane.key)?.peak_waiting || 0),
      borderColor: lane.queueColor,
      backgroundColor: lane.queueColor,
      borderDash: [6, 4],
      borderWidth: 1.5,
      pointRadius: 1,
      pointHitRadius: 10,
      tension: 0.25,
      userId: null,
      metric: 'waiting'
    }
  ]

  visibleUserIds(lane.key).forEach((userId, index) => {
    const color = palette[index % palette.length]
    datasets.push({
      label: userLabel(userId),
      data: points.map(point => pointUserLanePeak(point, userId, lane.key)?.peak_demand || 0),
      borderColor: color,
      backgroundColor: color,
      borderWidth: 1.25,
      pointRadius: 1,
      pointHitRadius: 10,
      tension: 0.25,
      userId,
      metric: 'demand'
    })
  })

  return {
    labels: points.map(point => formatMinute(point.bucket_start)),
    datasets
  }
}

function chartOptions(lane: LaneDefinition) {
  const dark = document.documentElement.classList.contains('dark')
  const textColor = dark ? '#9ca3af' : '#6b7280'
  const gridColor = dark ? '#374151' : '#e5e7eb'
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { intersect: false, mode: 'index' as const },
    plugins: {
      legend: {
        position: 'top' as const,
        align: 'start' as const,
        labels: { color: textColor, usePointStyle: true, boxWidth: 7, font: { size: 10 } }
      },
      tooltip: {
        backgroundColor: dark ? '#1f2937' : '#ffffff',
        titleColor: dark ? '#f3f4f6' : '#111827',
        bodyColor: dark ? '#d1d5db' : '#4b5563',
        borderColor: gridColor,
        borderWidth: 1,
        callbacks: {
          label: (context: any) => {
            const userId = context.dataset.userId as string | null
            if (!userId) return `${context.dataset.label}: ${context.parsed.y}`
            const point = trend.value?.points?.[context.dataIndex]
            const peak = point ? pointUserLanePeak(point, userId, lane.key) : undefined
            return t('admin.ops.concurrencyTrend.userTooltip', {
              user: userLabel(userId),
              demand: peak?.peak_demand || 0,
              active: peak?.peak_in_use || 0,
              waiting: peak?.peak_waiting || 0
            })
          }
        }
      }
    },
    scales: {
      x: {
        grid: { display: false },
        ticks: { color: textColor, maxTicksLimit: 8, font: { size: 10 } }
      },
      y: {
        beginAtZero: true,
        grid: { color: gridColor },
        ticks: { color: textColor, precision: 0, font: { size: 10 } },
        title: { display: true, text: t('admin.ops.concurrencyTrend.axis'), color: textColor, font: { size: 10 } }
      }
    }
  }
}

function pinSelectedUser() {
  const userId = selectedUserId.value
  if (!userId || pinnedUserIds.value.includes(userId)) return
  pinnedUserIds.value = [...pinnedUserIds.value, userId].slice(-5)
  selectedUserId.value = ''
}

function removePinnedUser(userId: string) {
  pinnedUserIds.value = pinnedUserIds.value.filter(id => id !== userId)
}

async function loadData() {
  loadController?.abort()
  const sequence = ++loadSequence
  const controller = new AbortController()
  loadController = controller
  loading.value = true
  errorMessage.value = ''
  try {
    const params: Parameters<typeof opsAPI.getUserConcurrencyTrend>[0] = {}
    if (props.timeRange === 'custom' && props.customStartTime && props.customEndTime) {
      params.start_time = props.customStartTime
      params.end_time = props.customEndTime
    } else if (props.timeRange !== 'custom') {
      params.time_range = props.timeRange as '5m' | '30m' | '1h' | '6h' | '24h'
    }
    const data = await opsAPI.getUserConcurrencyTrend(params, { signal: controller.signal })
    if (sequence !== loadSequence) return
    trend.value = data
    const validUsers = new Set(Object.keys(trend.value.users || {}))
    pinnedUserIds.value = pinnedUserIds.value.filter(userId => validUsers.has(userId))
  } catch (error: any) {
    if (sequence !== loadSequence || error?.code === 'ERR_CANCELED' || error?.name === 'AbortError') return
    console.error('[OpsUserConcurrencyTrend] Failed to load data', error)
    errorMessage.value = error?.response?.data?.detail || t('admin.ops.concurrencyTrend.loadFailed')
  } finally {
    if (sequence === loadSequence) {
      loadController = null
      loading.value = false
    }
  }
}

watch(() => props.refreshToken, loadData)
onMounted(loadData)
onBeforeUnmount(() => {
  loadSequence++
  loadController?.abort()
  loadController = null
})
</script>

<template>
  <div class="flex min-h-[500px] flex-col rounded-lg bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h3 class="text-sm font-bold text-gray-900 dark:text-white">{{ t('admin.ops.concurrencyTrend.title') }}</h3>
        <div class="mt-0.5 text-[11px] text-gray-500 dark:text-gray-400">{{ t('admin.ops.concurrencyTrend.subtitle') }}</div>
      </div>
      <div class="flex min-w-0 flex-1 items-center justify-end gap-2 sm:flex-none">
        <select
          v-model="selectedUserId"
          class="h-8 min-w-0 flex-1 rounded-lg border border-gray-200 bg-white px-2 text-xs text-gray-700 outline-none focus:border-blue-500 sm:w-[240px] sm:flex-none dark:border-dark-700 dark:bg-dark-900 dark:text-gray-200"
          :aria-label="t('admin.ops.concurrencyTrend.selectUser')"
          @change="pinSelectedUser"
        >
          <option value="">{{ t('admin.ops.concurrencyTrend.selectUser') }}</option>
          <option v-for="option in userOptions" :key="option.id" :value="option.id">
            {{ userOptionText(option.user, option.label) }}
          </option>
        </select>
        <button
          type="button"
          class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-600 transition-colors hover:bg-gray-200 disabled:opacity-50 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600"
          :disabled="loading"
          :title="t('common.refresh')"
          @click="loadData"
        >
          <svg class="h-3.5 w-3.5" :class="{ 'animate-spin': loading }" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
        </button>
      </div>
    </div>

    <div v-if="pinnedUserIds.length" class="mb-3 flex flex-wrap gap-1.5">
      <button
        v-for="userId in pinnedUserIds"
        :key="userId"
        type="button"
        class="inline-flex items-center gap-1 rounded bg-gray-100 px-2 py-1 text-[10px] font-medium text-gray-700 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600"
        :title="t('admin.ops.concurrencyTrend.removeUser')"
        @click="removePinnedUser(userId)"
      >
        <span class="max-w-[180px] truncate">{{ userLabel(userId) }}</span>
        <span aria-hidden="true">×</span>
      </button>
    </div>

    <div v-if="errorMessage" class="mb-3 rounded-lg bg-red-50 px-3 py-2 text-xs text-red-600 dark:bg-red-900/20 dark:text-red-400">
      {{ errorMessage }}
    </div>
    <div v-else-if="coverageWarning" class="mb-3 border-l-2 border-amber-500 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:bg-amber-950/30 dark:text-amber-300">
      {{ coverageWarning }}
    </div>

    <div class="grid min-h-0 flex-1 grid-cols-1 border-t border-gray-100 lg:grid-cols-3 dark:border-dark-700">
      <section
        v-for="(lane, index) in lanes"
        :key="lane.key"
        :data-lane="lane.key"
        class="min-w-0 py-4 lg:px-4"
        :class="{ 'border-t border-gray-100 lg:border-l lg:border-t-0 dark:border-dark-700': index > 0 }"
      >
        <div class="mb-2 flex items-center justify-between gap-2">
          <h4 class="text-xs font-semibold text-gray-800 dark:text-gray-100">{{ lane.title }}</h4>
          <div class="flex items-center gap-2 text-[10px] text-gray-500 dark:text-gray-400">
            <span>{{ t('admin.ops.concurrencyTrend.currentActive') }} <strong class="font-mono" :style="{ color: lane.color }">{{ currentLane(lane.key).in_use }}</strong></span>
            <span>{{ t('admin.ops.concurrencyTrend.currentWaiting') }} <strong class="font-mono text-red-600 dark:text-red-400">{{ currentLane(lane.key).waiting }}</strong></span>
          </div>
        </div>
        <div class="mb-3 border-y border-gray-100 dark:border-dark-700">
          <div
            class="py-2"
            :aria-label="`${lane.title} ${t('admin.ops.concurrencyTrend.demandStatistics')}`"
          >
            <div class="mb-1 px-1 text-[9px] font-semibold text-gray-400 dark:text-gray-500">
              {{ t('admin.ops.concurrencyTrend.demandStatistics') }}
            </div>
            <div class="grid grid-cols-5">
              <div
                v-for="statistic in statisticDefinitions"
                :key="statistic.key"
                class="min-w-0 text-center"
                data-metric="demand"
                :data-stat="statistic.key"
              >
                <div class="text-[9px] font-semibold uppercase text-gray-400 dark:text-gray-500">
                  {{ statistic.label }}
                </div>
                <div class="truncate font-mono text-xs font-semibold text-gray-800 dark:text-gray-100">
                  {{ formatStatistic(laneStatistics[lane.key][statistic.key]) }}
                </div>
              </div>
            </div>
          </div>
          <div
            class="border-t border-gray-100 py-2 dark:border-dark-700"
            :aria-label="`${lane.title} ${t('admin.ops.concurrencyTrend.responseTimeStatistics')}`"
          >
            <div class="mb-1 px-1 text-[9px] font-semibold text-gray-400 dark:text-gray-500">
              {{ t('admin.ops.concurrencyTrend.responseTimeStatistics') }}
            </div>
            <div class="grid grid-cols-5">
              <div
                v-for="statistic in statisticDefinitions"
                :key="statistic.key"
                class="min-w-0 text-center"
                data-metric="latency"
                :data-stat="statistic.key"
              >
                <div class="text-[9px] font-semibold uppercase text-gray-400 dark:text-gray-500">
                  {{ statistic.label }}
                </div>
                <div
                  class="truncate font-mono text-[10px] font-semibold text-gray-800 dark:text-gray-100"
                  :title="formatMilliseconds(laneLatencyStatistic(lane.key, statistic.key))"
                >
                  {{ formatMilliseconds(laneLatencyStatistic(lane.key, statistic.key)) }}
                </div>
              </div>
            </div>
          </div>
        </div>
        <div class="h-[300px] min-h-0">
          <Line v-if="hasLaneData(lane.key)" :data="laneChartData(lane)" :options="chartOptions(lane)" />
          <div v-else class="flex h-full items-center justify-center text-xs text-gray-400">
            {{ loading ? t('common.loading') : t('admin.ops.concurrencyTrend.laneEmpty') }}
          </div>
        </div>
      </section>
    </div>
  </div>
</template>
