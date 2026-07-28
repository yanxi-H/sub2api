<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { BarElement, CategoryScale, Chart as ChartJS, Legend, LinearScale, Tooltip } from 'chart.js'
import { Bar } from 'vue-chartjs'
import type { OpsPerformanceDiagnosticsResponse, OpsPerformanceImpact } from '@/api/admin/ops'
import { formatHistoryLabel } from '../utils/opsFormatters'

ChartJS.register(BarElement, CategoryScale, Legend, LinearScale, Tooltip)

type Dimension = 'user' | 'account' | 'model'

const props = defineProps<{
  data: OpsPerformanceDiagnosticsResponse | null
  loading: boolean
  timeRange: string
}>()

const { t } = useI18n()
const dimension = ref<Dimension>('user')

const summary = computed(() => props.data?.summary)
const causes = computed(() => props.data?.causes ?? [])
const maxCauseCount = computed(() => Math.max(...causes.value.map(item => item.count), 1))
const impacts = computed<OpsPerformanceImpact[]>(() =>
  (props.data?.impacts ?? []).filter(item => item.dimension === dimension.value)
)

const primaryCause = computed(() => causes.value[0])
const ingestionWarning = computed(() => {
  const health = props.data?.ingestion_health
  if (!health) return ''
  if (health.dropped_count > 0 || health.write_failed_count > 0) {
    return t('admin.ops.performance.ingestionLoss', {
      dropped: health.dropped_count,
      failed: health.write_failed_count
    })
  }
  if (health.queue_capacity > 0 && health.queue_depth / health.queue_capacity >= 0.8) {
    return t('admin.ops.performance.ingestionBacklog', {
      depth: health.queue_depth,
      capacity: health.queue_capacity
    })
  }
  return ''
})
const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))
const causeColors = ['#d97706', '#dc2626', '#2563eb', '#059669', '#7c3aed', '#0891b2', '#db2777', '#65a30d', '#ea580c', '#4f46e5', '#0d9488', '#6b7280']

const trendCauses = computed(() => {
  const totals = new Map<string, number>()
  for (const point of props.data?.trend ?? []) {
    for (const [cause, count] of Object.entries(point.causes ?? {})) {
      totals.set(cause, (totals.get(cause) ?? 0) + count)
    }
  }
  return [...totals.entries()].sort((a, b) => b[1] - a[1]).map(([cause]) => cause)
})

const historyRange = computed(() => {
  if (props.timeRange !== 'custom') return props.timeRange
  const start = new Date(props.data?.start_time ?? '').getTime()
  const end = new Date(props.data?.end_time ?? '').getTime()
  return Number.isFinite(start) && Number.isFinite(end) && end - start >= 24 * 60 * 60 * 1000 ? '24h' : '1h'
})

const trendChartData = computed(() => {
  const points = props.data?.trend ?? []
  if (!points.length || !trendCauses.value.length) return null
  return {
    labels: points.map(point => formatHistoryLabel(point.bucket_start, historyRange.value)),
    datasets: trendCauses.value.map((cause, index) => ({
      label: causeLabel(cause),
      data: points.map(point => point.causes?.[cause] ?? 0),
      backgroundColor: causeColors[index % causeColors.length],
      borderWidth: 0,
      stack: 'slow-causes',
      barPercentage: 0.92,
      categoryPercentage: 0.92
    }))
  }
})

const trendChartOptions = computed(() => {
  const textColor = isDarkMode.value ? '#9ca3af' : '#6b7280'
  const gridColor = isDarkMode.value ? '#374151' : '#f3f4f6'
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { intersect: false, mode: 'index' as const },
    plugins: {
      legend: {
        position: 'top' as const,
        align: 'end' as const,
        labels: { color: textColor, usePointStyle: true, boxWidth: 7, font: { size: 10 } }
      },
      tooltip: {
        backgroundColor: isDarkMode.value ? '#1f2937' : '#ffffff',
        titleColor: isDarkMode.value ? '#f3f4f6' : '#111827',
        bodyColor: isDarkMode.value ? '#d1d5db' : '#4b5563',
        borderColor: gridColor,
        borderWidth: 1,
        padding: 10
      }
    },
    scales: {
      x: {
        stacked: true,
        grid: { display: false },
        ticks: { color: textColor, font: { size: 10 }, maxTicksLimit: 10, autoSkip: true }
      },
      y: {
        stacked: true,
        beginAtZero: true,
        grid: { color: gridColor },
        ticks: { color: textColor, font: { size: 10 }, precision: 0 },
        title: { display: true, text: t('admin.ops.performance.slowRequestsAxis'), color: textColor, font: { size: 10 } }
      }
    }
  }
})

function formatMs(value?: number | null): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  return `${new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 }).format(value)} ms`
}

function formatPercent(value?: number | null): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  return `${value.toFixed(value >= 10 ? 1 : 2)}%`
}

function causeLabel(cause?: string): string {
  if (!cause) return '-'
  const key = `admin.ops.performance.causes.${cause}`
  const translated = t(key)
  return translated === key ? cause : translated
}

function impactName(item: OpsPerformanceImpact): string {
  return item.name || `#${item.id}`
}
</script>

<template>
  <section class="rounded-lg bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <div class="mb-4 flex flex-wrap items-start justify-between gap-3">
      <div>
        <h3 class="text-sm font-bold text-gray-900 dark:text-white">{{ t('admin.ops.performance.title') }}</h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          <template v-if="primaryCause">
            {{ t('admin.ops.performance.primaryCause', { cause: causeLabel(primaryCause.cause), share: formatPercent(primaryCause.share) }) }}
          </template>
          <template v-else>{{ t('admin.ops.performance.noSlowRequests') }}</template>
        </p>
      </div>
      <div class="grid grid-cols-3 gap-x-5 gap-y-1 text-right">
        <div>
          <div class="text-[10px] text-gray-400">E2E P95</div>
          <div class="font-mono text-sm font-semibold text-gray-900 dark:text-white">{{ formatMs(summary?.end_to_end?.p95_ms) }}</div>
        </div>
        <div>
          <div class="text-[10px] text-gray-400">{{ t('admin.ops.performance.endToEndTtftP95') }}</div>
          <div class="font-mono text-sm font-semibold text-gray-900 dark:text-white">{{ formatMs(summary?.ttft?.p95_ms) }}</div>
        </div>
        <div>
          <div class="text-[10px] text-gray-400">{{ t('admin.ops.performance.slowRate') }}</div>
          <div class="font-mono text-sm font-semibold text-gray-900 dark:text-white">{{ formatPercent(summary?.slow_rate) }}</div>
        </div>
      </div>
    </div>

    <div v-if="ingestionWarning" class="mb-4 border-l-2 border-amber-500 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:bg-amber-950/30 dark:text-amber-300">
      {{ ingestionWarning }}
    </div>

    <div v-if="loading && !data" class="flex h-56 items-center justify-center text-sm text-gray-400">
      {{ t('common.loading') }}
    </div>
    <div v-else class="space-y-6">
      <div class="min-w-0 border-t border-gray-100 pt-4 dark:border-dark-700">
        <h4 class="mb-3 text-xs font-semibold text-gray-700 dark:text-gray-200">{{ t('admin.ops.performance.causeTrend') }}</h4>
        <div class="h-48 min-w-0">
          <Bar v-if="trendChartData" :data="trendChartData" :options="trendChartOptions" />
          <div v-else class="flex h-full items-center justify-center text-xs text-gray-400">
            {{ t('admin.ops.performance.noSlowRequests') }}
          </div>
        </div>
      </div>

      <div class="grid grid-cols-1 gap-6 lg:grid-cols-[minmax(280px,0.8fr)_minmax(0,1.2fr)]">
        <div class="min-w-0 border-t border-gray-100 pt-4 dark:border-dark-700">
          <h4 class="mb-3 text-xs font-semibold text-gray-700 dark:text-gray-200">{{ t('admin.ops.performance.causeRanking') }}</h4>
          <div v-if="causes.length" class="space-y-3">
            <div v-for="item in causes" :key="item.cause">
              <div class="mb-1 flex items-center justify-between gap-3 text-xs">
                <span class="truncate font-medium text-gray-700 dark:text-gray-200">{{ causeLabel(item.cause) }}</span>
                <span class="shrink-0 font-mono text-gray-500">{{ item.count }} · {{ formatPercent(item.share) }}</span>
              </div>
              <div class="h-1.5 overflow-hidden rounded bg-gray-100 dark:bg-dark-700">
                <div class="h-full rounded bg-amber-500" :style="{ width: `${Math.max(3, item.count / maxCauseCount * 100)}%` }" />
              </div>
              <div class="mt-1 flex gap-3 text-[10px] text-gray-400">
                <span>E2E P95 {{ formatMs(item.e2e_p95_ms) }}</span>
                <span>{{ t('admin.ops.performance.queueP95') }} {{ formatMs(item.queue_p95_ms) }}</span>
                <span>{{ t('admin.ops.performance.endToEndTtftP95') }} {{ formatMs(item.ttft_p95_ms) }}</span>
              </div>
            </div>
          </div>
          <div v-else class="flex h-36 items-center justify-center text-xs text-gray-400">
            {{ t('admin.ops.performance.noSlowRequests') }}
          </div>
        </div>

        <div class="min-w-0 border-t border-gray-100 pt-4 dark:border-dark-700">
          <div class="mb-3 flex items-center justify-between gap-3">
            <h4 class="text-xs font-semibold text-gray-700 dark:text-gray-200">{{ t('admin.ops.performance.impactScope') }}</h4>
            <div class="inline-flex border border-gray-200 dark:border-dark-600">
              <button
                v-for="item in (['user', 'account', 'model'] as Dimension[])"
                :key="item"
                type="button"
                class="px-3 py-1 text-[11px] font-medium"
                :class="dimension === item ? 'bg-gray-900 text-white dark:bg-white dark:text-gray-900' : 'text-gray-500 hover:bg-gray-50 dark:hover:bg-dark-700'"
                @click="dimension = item"
              >
                {{ t(`admin.ops.performance.dimensions.${item}`) }}
              </button>
            </div>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full min-w-[720px] text-left text-xs">
              <thead class="text-[10px] uppercase text-gray-400">
                <tr class="border-b border-gray-100 dark:border-dark-700">
                  <th class="pb-2 font-medium">{{ t(`admin.ops.performance.dimensions.${dimension}`) }}</th>
                  <th class="pb-2 text-right font-medium">{{ t('admin.ops.performance.requests') }}</th>
                  <th class="pb-2 text-right font-medium">{{ t('admin.ops.performance.slowRate') }}</th>
                  <th class="pb-2 text-right font-medium">E2E P95</th>
                  <th class="pb-2 text-right font-medium">{{ t('admin.ops.performance.endToEndTtftP95') }}</th>
                  <th class="pb-2 text-right font-medium">{{ t('admin.ops.performance.queueP95') }}</th>
                  <th class="pb-2 text-right font-medium">{{ t('admin.ops.performance.mainCause') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in impacts" :key="`${item.dimension}:${item.id}`" class="border-b border-gray-50 dark:border-dark-700/60">
                  <td class="max-w-[220px] truncate py-2.5 font-medium text-gray-800 dark:text-gray-100" :title="impactName(item)">{{ impactName(item) }}</td>
                  <td class="py-2.5 text-right font-mono text-gray-500">{{ item.request_count }}</td>
                  <td class="py-2.5 text-right font-mono" :class="item.slow_rate > 10 ? 'text-red-600 dark:text-red-400' : 'text-gray-500'">{{ formatPercent(item.slow_rate) }}</td>
                  <td class="py-2.5 text-right font-mono text-gray-500">{{ formatMs(item.e2e_p95_ms) }}</td>
                  <td class="py-2.5 text-right font-mono text-gray-500">{{ formatMs(item.ttft_p95_ms) }}</td>
                  <td class="py-2.5 text-right font-mono text-gray-500">{{ formatMs(item.queue_p95_ms) }}</td>
                  <td class="py-2.5 text-right text-gray-600 dark:text-gray-300">{{ causeLabel(item.main_cause) }}</td>
                </tr>
                <tr v-if="!impacts.length">
                  <td colspan="7" class="h-28 text-center text-gray-400">{{ t('common.noData') }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
